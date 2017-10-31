package provider

import (
	//"strings"

	log "github.com/sirupsen/logrus"

	"github.com/kubernetes-incubator/external-dns/endpoint"
	"github.com/kubernetes-incubator/external-dns/plan"
	pgo "github.com/kubernetes-incubator/external-dns/provider/internal/pdns-go"
)

const (
	//recordTTL = 300
	HOST           = "http://devops-lab1.lab2-skae.tower-research.com:8088/api/v1"
	DEFAULT_SERVER = "localhost"
	PDNS_TOKEN     = "pdns"
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
	for _, change := range changes.Create {
		log.Debugf("CREATE: %+v", change)
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
