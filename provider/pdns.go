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
	HOST           = "http://devops-lab1.lab2-skae.tower-research.com:8088"
	DEFAULT_SERVER = "locahost"
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

// Records returns the list of records in a given hosted zone.
func (p *PDNSProvider) Records() (endpoints []*endpoint.Endpoint, _ error) {
	a := pgo.NewZonesApiWithBasePath(HOST)
	zones, _, err := a.ListZones(DEFAULT_SERVER)
	if err != nil {
		log.Warnf("Unable to fetch zones from %s. %v", HOST, err)
		return nil, err
	}

	for _, zone := range zones {
		log.Debugf("zone: %v", zone)
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
