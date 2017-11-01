package provider

import (
	//"strings"
	"errors"

	log "github.com/sirupsen/logrus"

	"github.com/kubernetes-incubator/external-dns/endpoint"
	"github.com/kubernetes-incubator/external-dns/plan"
	pgo "github.com/kubernetes-incubator/external-dns/provider/internal/pdns-go"
)

const (
	DEFAULT_TTL    = 300
	HOST           = "http://devops-lab1.lab2-skae.tower-research.com:8088/api/v1" // this should be a cli param as well
	DEFAULT_SERVER = "localhost"
	PDNS_TOKEN     = "pdns" // TODO: This should be a cli param
	MAX_UINT32     = ^uint32(0)
	MAX_INT32      = MAX_UINT32 >> 1
	DEFAULT_ZONE   = "kube-test.skae.tower-research.com."
)

type PDNSProvider struct {
	//client Route53API
	dryRun bool
	// only consider hosted zones managing domains ending in this suffix
	domainFilter DomainFilter
	// filter hosted zones by type (e.g. private or public)
	zoneTypeFilter ZoneTypeFilter
}

// Zones returns the list of hosted zones.
/*
func (p *PDNSProvider) Zones() (map[string]*route53.HostedZone, error) {
	//zones := make(map[string]*route53.HostedZone)
	return zones, nil
}
*/

func NewPDNSProvider() (*PDNSProvider, error) {

	provider := &PDNSProvider{}

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

// Records returns the list of records in a given hosted zone.
func (p *PDNSProvider) Records() (endpoints []*endpoint.Endpoint, _ error) {
	a := pgo.NewZonesApiWithBasePath(HOST)
	a.Configuration.APIKey["X-API-Key"] = PDNS_TOKEN
	//log.Debugf("%+v", a)
	//log.Debugf("X-API-Key: %+v", a.Configuration.GetAPIKeyWithPrefix("X-API-Key"))
	//zones, resp, err := a.ListZones(DEFAULT_SERVER)
	zones, _, err := a.ListZones(DEFAULT_SERVER)
	log.Debugf("Zones: %+v", zones)
	//#log.Debugf("Response: %+v", resp)
	//#log.Debugf("Response: %+v", string(resp.Payload))
	if err != nil {
		log.Warnf("Unable to fetch zones from %s. %v", HOST, err)
		return nil, err
	}

	for _, zone := range zones {
		log.Debugf("zone: %+v", zone)
		z, _, err := a.ListZone(DEFAULT_SERVER, zone.Id)
		if err != nil {
			log.Warnf("Unable to fetch data for %v from %s. %v", zone.Id, HOST, err)
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
			log.Debugf("Endpoints: %+v", endpoints)
		}
	}

	return endpoints, nil
}

func (p *PDNSProvider) ApplyChanges(changes *plan.Changes) error {
	a := pgo.NewZonesApiWithBasePath(HOST)
	a.Configuration.APIKey["X-API-Key"] = PDNS_TOKEN

	for _, change := range changes.Create {
		log.Debugf("CREATE: %+v", change)
		rrset, err := EndpointToRRSet(change)
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

		//z, _, err := a.ListZone(DEFAULT_SERVER, zone.Id, zone)
		resp, err := a.PatchZone(DEFAULT_SERVER, zone.Id, zone)
		log.Debugf("[PatchZone] resp: %+v", resp)
		log.Debugf("[PatchZone] resp.Payload: %+v", string(resp.Payload))
		if err != nil {
			return err
		}
	}

	for _, change := range changes.UpdateOld {
		log.Debugf("UPDATE-OLD: %+v", change)
	}

	for _, change := range changes.UpdateNew {
		log.Debugf("UPDATE-NEW: %+v", change)
	}

	for _, change := range changes.Delete {
		log.Debugf("DELETE: %+v", change)
	}
	return nil
}
