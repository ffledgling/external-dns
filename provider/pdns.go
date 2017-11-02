package provider

import (
	//"strings"
	"bytes"
	"context"
	"errors"
	"net/http"

	log "github.com/sirupsen/logrus"

	"github.com/kubernetes-incubator/external-dns/endpoint"
	"github.com/kubernetes-incubator/external-dns/plan"
	pgo "github.com/kubernetes-incubator/external-dns/provider/internal/pdns-go"
)

const (
	API_BASE = "/api/v1"

	DEFAULT_SERVER = "localhost" // We only talk to the Authoritative server, so this is always localhost
	DEFAULT_TTL    = 300
	DEFAULT_ZONE   = "kube-test.skae.tower-research.com." // TODO: We should be calculating which zone we need to insert a record into

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

func EndpointsToRRSets(endpoints []*endpoint.Endpoint) (rrsets []pgo.RrSet, _ error) {
	// TODO: ffledgling

	return rrsets, nil
}

func EndpointToRRSet(e *endpoint.Endpoint) (rr pgo.RrSet, _ error) {
	// Theoretically an RRSet encapsulates more than one record of the same type.
	// For example multiple A records are clubbed under the same RRSet
	// Using an RRSet that encapsulates only one endpoint (record in PDNS terms)
	// and then using it to patch via PDNS API might result in lost records

	rr.Name = ensureTrailingDot(e.DNSName)
	// Check we don't cause an overflow by typecasting int64 to int32
	if int64(e.RecordTTL) > int64(MAX_INT32) {
		return rr, errors.New("Value of record TTL overflows, limited to int32")
	}
	var TTL int32
	if e.RecordTTL == 0 {
		TTL = DEFAULT_TTL
	} else {
		TTL = int32(e.RecordTTL)
	}
	rr.Ttl = TTL
	rr.Type_ = e.RecordType
	rr.Records = append(rr.Records, pgo.Record{Content: e.Target, Disabled: false, SetPtr: false})

	return rr, nil
}
func (p *PDNSProvider) DeleteRecord(endpoint *endpoint.Endpoint) error {
	rrset, err := EndpointToRRSet(endpoint)
	if err != nil {
		return err
	}
	// Necessary for deleting records
	rrset.Changetype = "DELETE"
	log.Debugf("[EndpointToRRSet] RRSet: %+v", rrset)
	zone := pgo.Zone{}
	zone.Name = "kube-test.skae.tower-research.com."
	zone.Id = "kube-test.skae.tower-research.com."
	zone.Kind = "Native"
	rrsets := []pgo.RrSet{rrset}
	zone.Rrsets = rrsets

	resp, err := p.client.ZonesApi.PatchZone(p.auth_ctx, DEFAULT_SERVER, zone.Id, zone)
	log.Debugf("[PatchZone] resp: %+v", resp)
	log.Debugf("[PatchZone] resp.Body: %+v", printHTTPResponseBody(resp))
	if err != nil {
		return err
	}

	return nil
}

func (p *PDNSProvider) ReplaceRecord(endpoint *endpoint.Endpoint) error {
	rrset, err := EndpointToRRSet(endpoint)
	if err != nil {
		return err
	}
	// Necessary for creating or modifying records
	rrset.Changetype = "REPLACE"
	log.Debugf("[EndpointToRRSet] RRSet: %+v", rrset)
	zone := pgo.Zone{}
	zone.Name = "kube-test.skae.tower-research.com."
	zone.Id = "kube-test.skae.tower-research.com."
	zone.Kind = "Native"
	rrsets := []pgo.RrSet{rrset}
	zone.Rrsets = rrsets

	resp, err := p.client.ZonesApi.PatchZone(p.auth_ctx, DEFAULT_SERVER, zone.Id, zone)
	log.Debugf("[PatchZone] resp: %+v", resp)
	log.Debugf("[PatchZone] resp.Body: %+v", printHTTPResponseBody(resp))
	if err != nil {
		return err
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
		log.Debugf("zone: %+v", zone)
		z, _, err := p.client.ZonesApi.ListZone(p.auth_ctx, DEFAULT_SERVER, zone.Id)
		if err != nil {
			log.Warnf("Unable to fetch data for %v. %v", zone.Id, err)
			return nil, err
		}

		log.Debugf("zone data: %+v", z)
		for _, rr := range z.Rrsets {
			log.Debugf("rrset: %+v", rr)
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
		p.ReplaceRecord(change)
	}

	for _, change := range changes.UpdateOld {
		log.Debugf("UPDATE-OLD: %+v", change)
		// Since PDNS "Patches", we don't need to specify the "old" record.
		// The Update New change type will automatically take care of replacing the old RRSet with the new one
	}

	for _, change := range changes.UpdateNew {
		log.Debugf("UPDATE-NEW: %+v", change)
		p.ReplaceRecord(change)
	}

	for _, change := range changes.Delete {
		log.Debugf("DELETE: %+v", change)
		p.DeleteRecord(change)
	}
	return nil
}
