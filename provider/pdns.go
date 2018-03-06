package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"

	log "github.com/sirupsen/logrus"

	pgo "github.com/ffledgling/pdns-go"
	"github.com/kubernetes-incubator/external-dns/endpoint"
	"github.com/kubernetes-incubator/external-dns/plan"
)

type pdnsChangeType string

const (
	apiBase = "/api/v1"

	// Unless we use something like pdnsproxy (discontinued upsteam), this value will _always_ be localhost
	defaultServerID = "localhost"
	defaultTTL      = 300

	// This is effectively an enum for "pgo.RrSet.changetype"
	// TODO: Can we somehow get this from the pgo swagger client library itself?
	pdnsDelete  pdnsChangeType = "DELETE"
	pdnsReplace pdnsChangeType = "REPLACE"
)

// Function for debug printing
func stringifyHTTPResponseBody(r *http.Response) (body string) {

	if r == nil {
		return ""
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(r.Body)
	body = buf.String()
	return body

}

// All of this interface and inherited client struct requires
type PDNSAPIProvider interface {
	ListZones() ([]pgo.Zone, *http.Response, error)
	ListZone(zoneId string) (pgo.Zone, *http.Response, error)
	PatchZone(zoneId string, zoneStruct pgo.Zone) (*http.Response, error)
}

type PDNSAPIClient struct {
	dryRun  bool
	authCtx context.Context
	client  *pgo.APIClient
}

func (c *PDNSAPIClient) ListZones() ([]pgo.Zone, *http.Response, error) {
	zones, resp, err := c.client.ZonesApi.ListZones(c.authCtx, defaultServerID)
	if err != nil {
		log.Warnf("Unable to fetch zones. %v", err)
	}
	return zones, resp, err
}

func (c *PDNSAPIClient) ListZone(zoneId string) (pgo.Zone, *http.Response, error) {
	zones, resp, err := c.client.ZonesApi.ListZone(c.authCtx, defaultServerID, zoneId)
	if err != nil {
		log.Warnf("Unable to list zone %s. %v", zoneId, err)
	}
	return zones, resp, err
}

func (c *PDNSAPIClient) PatchZone(zoneId string, zoneStruct pgo.Zone) (*http.Response, error) {
	resp, err := c.client.ZonesApi.PatchZone(c.authCtx, defaultServerID, zoneId, zoneStruct)
	if err != nil {
		log.Warnf("Unable to patch zone %s. %v", zoneStruct.Name, err)
	}
	return resp, err
}

// PDNSProvider is an implementation of the Provider interface for PowerDNS
type PDNSProvider struct {
	client PDNSAPIProvider
}

// NewPDNSProvider initializes a new PowerDNS based Provider.
func NewPDNSProvider(server string, apikey string, domainFilter DomainFilter, dryRun bool) (*PDNSProvider, error) {

	// Do some input validation

	if apikey == "" {
		return nil, errors.New("Missing API Key for PDNS. Specify using --pdns-api-key=")
	}

	// The default for when no --domain-filter is passed is [""], instead of [], so we check accordingly.
	if len(domainFilter.filters) != 1 && domainFilter.filters[0] != "" {
		return nil, errors.New("PDNS Provider does not support domain filter.")
	}
	// We do not support dry running, exit safely instead of surprising the user
	// TODO: Add Dry Run support
	if dryRun {
		return nil, errors.New("PDNS Provider does not currently support dry-run.")
	}

	if server == "localhost" {
		log.Warnf("PDNS Server is set to localhost, this may not be what you want. Specify using --pdns-server=")
	}

	cfg := pgo.NewConfiguration()
	cfg.Host = server
	cfg.BasePath = server + apiBase

	provider := &PDNSProvider{
		client: &PDNSAPIClient{
			dryRun:  dryRun,
			authCtx: context.WithValue(context.TODO(), pgo.ContextAPIKey, pgo.APIKey{Key: apikey}),
			client:  pgo.NewAPIClient(cfg),
		},
	}

	return provider, nil
}

func (p *PDNSProvider) convertRRSetToEndpoints(rr pgo.RrSet) (endpoints []*endpoint.Endpoint, _ error) {
	endpoints = []*endpoint.Endpoint{}

	for _, record := range rr.Records {
		// If a record is "Disabled", it's not supposed to be "visible"
		if !record.Disabled {
			endpoints = append(endpoints, endpoint.NewEndpointWithTTL(rr.Name, record.Content, rr.Type_, endpoint.TTL(rr.Ttl)))
		}
	}

	return endpoints, nil
}

// convertEndpointsToZones marshals endpoints into pdns compatible Zone structs
func (p *PDNSProvider) ConvertEndpointsToZones(endpoints []*endpoint.Endpoint, changetype pdnsChangeType) (zonelist []pgo.Zone, _ error) {
	/* eg of mastermap
	    { "example.com":
		{ "app.example.com":
		    { "A": ["192.168.0.1", "8.8.8.8"] }
		    { "TXT": ["\"heritage=external-dns,external-dns/owner=example\""] }
		}
	    }
	*/
	mastermap := make(map[string]map[string]map[string][]*endpoint.Endpoint)
	zoneNameStructMap := map[string]pgo.Zone{}

	zones, _, err := p.client.ListZones()
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

		if zname == "" {
			return []pgo.Zone{}, errors.New(fmt.Sprintf("No matching zone found for %+v", ep))
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

	for zname := range mastermap {

		zone := zoneNameStructMap[zname]
		zone.Rrsets = []pgo.RrSet{}

		// Sort for deterministic struct
		rrnames := make([]string, 0, len(mastermap[zname]))
		for k := range mastermap[zname] {
			rrnames = append(rrnames, k)
		}
		sort.Strings(rrnames)

		for _, rrname := range rrnames {
			// Sort for deterministic struct
			rtypes := make([]string, 0, len(mastermap[zname][rrname]))
			for k := range mastermap[zname][rrname] {
				rtypes = append(rtypes, k)
			}
			sort.Strings(rtypes)
			for _, rtype := range rtypes {
				rrset := pgo.RrSet{}
				rrset.Name = rrname
				rrset.Type_ = rtype
				rrset.Changetype = string(changetype)
				rttl := mastermap[zname][rrname][rtype][0].RecordTTL
				if int64(rttl) > int64(math.MaxInt32) {
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
			zonelist = append(zonelist, zone)
		}

	}

	log.Debugf("Zone List generated from Endpoints: %+v", zonelist)

	return zonelist, nil
}

// mutateRecords takes a list of endpoints and creates, replaces or deletes them based on the changetype
func (p *PDNSProvider) mutateRecords(endpoints []*endpoint.Endpoint, changetype pdnsChangeType) error {
	zonelist, err := p.ConvertEndpointsToZones(endpoints, changetype)
	if err != nil {
		return err
	}
	for _, zone := range zonelist {
		jso, err := json.Marshal(zone)
		if err != nil {
			log.Debugf("JSON Marshal for zone struct failed!")
		} else {
			log.Debugf("Struct for PatchZone:\n%s", string(jso))
		}

		resp, err := p.client.PatchZone(zone.Id, zone)
		if err != nil {
			log.Debugf("PDNS API response: %s", stringifyHTTPResponseBody(resp))
			return err
		}

	}
	return nil
}

// Records returns all DNS records controlled by the configured PDNS server (for all zones)
func (p *PDNSProvider) Records() (endpoints []*endpoint.Endpoint, _ error) {

	zones, _, err := p.client.ListZones()
	if err != nil {
		return nil, err
	}

	for _, zone := range zones {
		z, _, err := p.client.ListZone(zone.Id)
		if err != nil {
			log.Warnf("Unable to fetch Records")
			return nil, err
		}

		for _, rr := range z.Rrsets {
			e, err := p.convertRRSetToEndpoints(rr)
			if err != nil {
				return nil, err
			}
			endpoints = append(endpoints, e...)
		}
	}

	log.Debugf("Records fetched:\n%+v", endpoints)
	return endpoints, nil
}

// ApplyChanges takes a list of changes (endpoints) and updates the PDNS server
// by sending the correct HTTP PATCH requests to a matching zone
func (p *PDNSProvider) ApplyChanges(changes *plan.Changes) error {

	// Create
	for _, change := range changes.Create {
		log.Debugf("CREATE: %+v", change)
	}
	// We only attempt to mutate records if there are any to mutate.  A
	// call to mutate records with an empty list of endpoints is still a
	// valid call and a no-op, but we might as well not make the call to
	// prevent unnecessary logging
	if len(changes.Create) > 0 {
		// "Replacing" non-existant records creates them
		p.mutateRecords(changes.Create, pdnsReplace)
	}

	// Update
	for _, change := range changes.UpdateOld {
		// Since PDNS "Patches", we don't need to specify the "old"
		// record. The Update New change type will automatically take
		// care of replacing the old RRSet with the new one We simply
		// leave this logging here for information
		log.Debugf("UPDATE-OLD (ignored): %+v", change)
	}

	for _, change := range changes.UpdateNew {
		log.Debugf("UPDATE-NEW: %+v", change)
	}
	if len(changes.UpdateNew) > 0 {
		p.mutateRecords(changes.UpdateNew, pdnsReplace)
	}

	// Delete
	for _, change := range changes.Delete {
		log.Debugf("DELETE: %+v", change)
	}
	if len(changes.Delete) > 0 {
		p.mutateRecords(changes.Delete, pdnsDelete)
	}
	return nil
}
