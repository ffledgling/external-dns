package provider

import (
	//"strings"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/kubernetes-incubator/external-dns/endpoint"
	"github.com/kubernetes-incubator/external-dns/plan"
	pgo "github.com/kubernetes-incubator/external-dns/provider/internal/pdns-go"
)

const (
	API_BASE = "/api/v1"

	DEFAULT_SERVER = "localhost" // We only talk to the Authoritative server, so this is always localhost
	DEFAULT_TTL    = 300

	MAX_UINT32 = ^uint32(0)
	MAX_INT32  = MAX_UINT32 >> 1
)

type PDNSProvider struct {
	//client Route53API
	dryRun bool
	// only consider hosted zones managing domains ending in this suffix
	domainFilter DomainFilter
	// filter hosted zones by type (e.g. private or public)
	zoneTypeFilter ZoneTypeFilter

	// Swagger API Client
	client *pgo.APIClient

	// Auth context to be passed to client requests, contains API keys etc.
	auth_ctx context.Context
}

// This function is just for debugging
func printHTTPResponseBody(r *http.Response) (body string) {

	buf := new(bytes.Buffer)
	buf.ReadFrom(r.Body)
	body = buf.String()
	return body

}

func NewPDNSProvider(server string, apikey string, domainFilter DomainFilter, dryRun bool) (*PDNSProvider, error) {

	// Do some input validation

	// We do not support dry running, exit safely instead of surprising the user
	// TODO: Add Dry Run support
	if dryRun {
		log.Fatalf("PDNS Provider does not currently support dry-run, stopping.")
	}

	if server == "localhost" {
		log.Warnf("PDNS Server is set to localhost, this is likely not what you want. Specify using --pdns-server=")
	}

	if apikey == "" {
		log.Warnf("API Key for PDNS is empty. Specify using --pdns-api-key=")
	}
	if len(domainFilter.filters) == 0 {
		log.Warnf("Domain Filter is not supported by PDNS. It will be ignored.")
	}

	provider := &PDNSProvider{}

	cfg := pgo.NewConfiguration()
	cfg.Host = server
	cfg.BasePath = server + API_BASE

	// Initialize a single client that we can use for all requests
	provider.client = pgo.NewAPIClient(cfg)

	// Configure PDNS API Key, which is sent via X-API-Key header to pdns server
	provider.auth_ctx = context.WithValue(context.TODO(), pgo.ContextAPIKey, pgo.APIKey{Key: apikey})

	return provider, nil
}

func convertRRSetToEndpoints(rr pgo.RrSet) (endpoints []*endpoint.Endpoint, _ error) {
	endpoints = []*endpoint.Endpoint{}

	for _, record := range rr.Records {
		//func NewEndpointWithTTL(dnsName, target, recordType string, ttl TTL) *Endpoint
		endpoints = append(endpoints, endpoint.NewEndpointWithTTL(rr.Name, record.Content, rr.Type_, endpoint.TTL(rr.Ttl)))
	}

	return endpoints, nil
}

func (p *PDNSProvider) Zones() (zones []pgo.Zone, _ error) {
	zones, _, err := p.client.ZonesApi.ListZones(p.auth_ctx, DEFAULT_SERVER)
	if err != nil {
		log.Warnf("Unable to fetch zones. %v", err)
		return nil, err
	}

	return zones, nil

}

func (p *PDNSProvider) EndpointsToZones(endpoints []*endpoint.Endpoint, changetype string) (zonelist []pgo.Zone, _ error) {
	// TODO: ffledgling
	// To convert to RRSets, we need to club changesets by zone first, then by name, then type
	/* eg.
	    { "example.com":
		{ "app.example.com":
		    { "A": ["192.168.0.1", "8.8.8.8"] }
		    { "TXT": ["\"heritage=external-dns,external-dns/owner=example\""] }
		}
	    }
	*/
	mastermap := make(map[string]map[string]map[string][]*endpoint.Endpoint)
	zoneNameStructMap := map[string]pgo.Zone{}

	zones, err := p.Zones()

	if err != nil {
		return nil, err
	}
	// Identify zones we control
	for _, z := range zones {
		mastermap[z.Name] = make(map[string]map[string][]*endpoint.Endpoint)
		zoneNameStructMap[z.Name] = z
	}

	for _, ep := range endpoints {
		// Identify which zone an endpoint belongs to
		dnsname := ensureTrailingDot(ep.DNSName)
		zname := ""
		for z := range mastermap {
			if strings.HasSuffix(dnsname, z) && len(dnsname) > len(zname) {
				zname = z
			}
		}

		// We can encounter a DNS name multiple times (different record types), we only create a map the first time
		if _, ok := mastermap[zname][dnsname]; !ok {
			mastermap[zname][dnsname] = make(map[string][]*endpoint.Endpoint)
		}

		// We can get multiple targets for the same record type (eg. Multiple A records for a service)
		if _, ok := mastermap[zname][dnsname][ep.RecordType]; !ok {
			mastermap[zname][dnsname][ep.RecordType] = make([]*endpoint.Endpoint, 0)
		}

		mastermap[zname][dnsname][ep.RecordType] = append(mastermap[zname][dnsname][ep.RecordType], ep)

	}

	log.Debugf("Conversion Map: %+v", mastermap)

	for zname := range mastermap {

		zone := zoneNameStructMap[zname]
		zone.Rrsets = []pgo.RrSet{}
		for rrname := range mastermap[zname] {
			for rtype := range mastermap[zname][rrname] {
				rrset := pgo.RrSet{}
				rrset.Name = rrname
				rrset.Type_ = rtype
				rrset.Changetype = changetype
				// TODO: We should check the typecasting here because we're down casting from int64 to int32
				rttl := mastermap[zname][rrname][rtype][0].RecordTTL
				if int64(rttl) > int64(MAX_INT32) {
					return nil, errors.New("Value of record TTL overflows, limited to int32")
				}
				rrset.Ttl = int32(rttl)
				records := []pgo.Record{}
				for _, e := range mastermap[zname][rrname][rtype] {
					records = append(records, pgo.Record{Content: e.Target})

				}
				rrset.Records = records
				zone.Rrsets = append(zone.Rrsets, rrset)
			}

		}

		// Skip the empty zones (likely ones we don't control)
		if len(zone.Rrsets) > 0 {
			jso, _ := json.Marshal(zone)
			log.Debugf("JSON of Zone:\n%s", string(jso))
			zonelist = append(zonelist, zone)
		}

	}

	log.Debugf("Conversion ZoneList: %+v", zonelist)

	return zonelist, nil
}

func (p *PDNSProvider) DeleteRecords(endpoints []*endpoint.Endpoint) error {
	if zonelist, err := p.EndpointsToZones(endpoints, "DELETE"); err != nil {
		return err
	} else {
		for _, zone := range zonelist {
			resp, err := p.client.ZonesApi.PatchZone(p.auth_ctx, DEFAULT_SERVER, zone.Id, zone)
			log.Debugf("Patch response: %s", printHTTPResponseBody(resp))
			if err != nil {
				return err
			}

		}
	}
	return nil
}

func (p *PDNSProvider) ReplaceRecords(endpoints []*endpoint.Endpoint) error {

	if zonelist, err := p.EndpointsToZones(endpoints, "REPLACE"); err != nil {
		return err
	} else {
		for _, zone := range zonelist {
			resp, err := p.client.ZonesApi.PatchZone(p.auth_ctx, DEFAULT_SERVER, zone.Id, zone)
			log.Debugf("Patch response: %s", printHTTPResponseBody(resp))
			if err != nil {
				return err
			}

		}
	}
	return nil
}

// Records returns the list of records in a given hosted zone.
func (p *PDNSProvider) Records() (endpoints []*endpoint.Endpoint, _ error) {

	zones, _, err := p.client.ZonesApi.ListZones(p.auth_ctx, DEFAULT_SERVER)
	if err != nil {
		log.Warnf("Unable to fetch zones from %v", err)
		return nil, err
	}

	for _, zone := range zones {
		//log.Debugf("zone: %+v", zone)
		z, _, err := p.client.ZonesApi.ListZone(p.auth_ctx, DEFAULT_SERVER, zone.Id)
		if err != nil {
			log.Warnf("Unable to fetch data for %v. %v", zone.Id, err)
			return nil, err
		}

		//log.Debugf("zone data: %+v", z)
		for _, rr := range z.Rrsets {
			//log.Debugf("rrset: %+v", rr)
			e, err := convertRRSetToEndpoints(rr)
			if err != nil {
				return nil, err
			}
			endpoints = append(endpoints, e...)
			//log.Debugf("Endpoints: %+v", endpoints)
		}
	}

	return endpoints, nil
}

func (p *PDNSProvider) ApplyChanges(changes *plan.Changes) error {

	for _, change := range changes.Create {
		log.Debugf("CREATE: %+v", change)
		//p.ReplaceRecord(change)
	}

	p.ReplaceRecords(changes.Create)

	for _, change := range changes.UpdateOld {
		// Since PDNS "Patches", we don't need to specify the "old" record.
		// The Update New change type will automatically take care of replacing the old RRSet with the new one
		log.Debugf("UPDATE-OLD: %+v", change)
	}

	for _, change := range changes.UpdateNew {
		log.Debugf("UPDATE-NEW: %+v", change)
	}
	p.ReplaceRecords(changes.UpdateNew)

	for _, change := range changes.Delete {
		log.Debugf("DELETE: %+v", change)
		//p.DeleteRecord(change)
	}
	p.DeleteRecords(changes.Delete)
	return nil
}
