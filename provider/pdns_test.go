package provider

import (
	//"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	//"github.com/stretchr/testify/require"

	pgo "github.com/ffledgling/pdns-go"
	"github.com/kubernetes-incubator/external-dns/endpoint"
)

// Test when we create a new provider it sets certain values correctly and errors out correctly in certain cases.

// Test zones to endpoints works correctly
// Test endpoints to zones works correctly
// Test the correct list/patch functions are called?
// Check your "regular"/80% case for arguments

// FIXME: Disabled record should not be returned in .Records()
// FIXME: What do we do about labels?

var (
	// Simple RRSets that contain 1 A record and 1 TXT record
	RRSetSimpleARecord = pgo.RrSet{
		Name:  "example.com.",
		Type_: "A",
		Ttl:   300,
		Records: []pgo.Record{
			pgo.Record{Content: "8.8.8.8", Disabled: false, SetPtr: false},
		},
	}
	RRSetSimpleTXTRecord = pgo.RrSet{
		Name:  "example.com.",
		Type_: "TXT",
		Ttl:   300,
		Records: []pgo.Record{
			pgo.Record{Content: "\"heritage=external-dns,external-dns/owner=tower-pdns\"", Disabled: false, SetPtr: false},
		},
	}
	endpointsSimpleRecord = []*endpoint.Endpoint{
		endpoint.NewEndpointWithTTL("example.com", "8.8.8.8", endpoint.RecordTypeA, endpoint.TTL(300)),
		endpoint.NewEndpointWithTTL("example.com", "\"heritage=external-dns,external-dns/owner=tower-pdns\"", endpoint.RecordTypeTXT, endpoint.TTL(300)),
	}

	endpointsNonexistantZone = []*endpoint.Endpoint{
		endpoint.NewEndpointWithTTL("does.not.exist.com", "8.8.8.8", endpoint.RecordTypeA, endpoint.TTL(300)),
		endpoint.NewEndpointWithTTL("does.not.exist.com", "\"heritage=external-dns,external-dns/owner=tower-pdns\"", endpoint.RecordTypeTXT, endpoint.TTL(300)),
	}

	// RRSet with one record disabled
	RRSetDisabledRecord = pgo.RrSet{
		Name:  "example.com.",
		Type_: "A",
		Ttl:   300,
		Records: []pgo.Record{
			pgo.Record{Content: "8.8.8.8", Disabled: false, SetPtr: false},
			pgo.Record{Content: "8.8.4.4", Disabled: true, SetPtr: false},
		},
	}
	endpointsDisabledRecord = []*endpoint.Endpoint{
		endpoint.NewEndpointWithTTL("example.com", "8.8.8.8", endpoint.RecordTypeA, endpoint.TTL(300)),
	}

	RRSetCNAMERecord = pgo.RrSet{
		Name:  "cname.example.com.",
		Type_: "CNAME",
		Ttl:   300,
		Records: []pgo.Record{
			pgo.Record{Content: "example.by.any.other.name.com", Disabled: false, SetPtr: false},
		},
	}
	RRSetTXTRecord = pgo.RrSet{
		Name:  "example.com.",
		Type_: "TXT",
		Ttl:   300,
		Records: []pgo.Record{
			pgo.Record{Content: "'would smell as sweet'", Disabled: false, SetPtr: false},
		},
	}

	// Multiple PDNS records in an RRSet of a single type
	RRSetMultipleRecords = pgo.RrSet{
		Name:  "example.com.",
		Type_: "A",
		Ttl:   300,
		Records: []pgo.Record{
			pgo.Record{Content: "8.8.8.8", Disabled: false, SetPtr: false},
			pgo.Record{Content: "8.8.4.4", Disabled: false, SetPtr: false},
			pgo.Record{Content: "4.4.4.4", Disabled: false, SetPtr: false},
		},
	}

	endpointsMultipleRecords = []*endpoint.Endpoint{
		endpoint.NewEndpointWithTTL("example.com", "8.8.8.8", endpoint.RecordTypeA, endpoint.TTL(300)),
		endpoint.NewEndpointWithTTL("example.com", "8.8.4.4", endpoint.RecordTypeA, endpoint.TTL(300)),
		endpoint.NewEndpointWithTTL("example.com", "4.4.4.4", endpoint.RecordTypeA, endpoint.TTL(300)),
	}

	endpointsMixedRecords = []*endpoint.Endpoint{
		endpoint.NewEndpointWithTTL("cname.example.com", "example.by.any.other.name.com", endpoint.RecordTypeCNAME, endpoint.TTL(300)),
		endpoint.NewEndpointWithTTL("example.com", "'would smell as sweet'", endpoint.RecordTypeTXT, endpoint.TTL(300)),
		endpoint.NewEndpointWithTTL("example.com", "8.8.8.8", endpoint.RecordTypeA, endpoint.TTL(300)),
		endpoint.NewEndpointWithTTL("example.com", "8.8.4.4", endpoint.RecordTypeA, endpoint.TTL(300)),
		endpoint.NewEndpointWithTTL("example.com", "4.4.4.4", endpoint.RecordTypeA, endpoint.TTL(300)),
	}

	endpointsMultipleZones = []*endpoint.Endpoint{
		endpoint.NewEndpointWithTTL("example.com", "8.8.8.8", endpoint.RecordTypeA, endpoint.TTL(300)),
		endpoint.NewEndpointWithTTL("example.com", "\"heritage=external-dns,external-dns/owner=tower-pdns\"", endpoint.RecordTypeTXT, endpoint.TTL(300)),
		endpoint.NewEndpointWithTTL("mock.test", "9.9.9.9", endpoint.RecordTypeA, endpoint.TTL(300)),
		endpoint.NewEndpointWithTTL("mock.test", "\"heritage=external-dns,external-dns/owner=tower-pdns\"", endpoint.RecordTypeTXT, endpoint.TTL(300)),
	}

	//
	ZoneEmpty = pgo.Zone{
		// Opaque zone id (string), assigned by the server, should not be interpreted by the application. Guaranteed to be safe for embedding in URLs.
		Id: "example.com.",
		// Name of the zone (e.g. “example.com.”) MUST have a trailing dot
		Name: "example.com.",
		// Set to “Zone”
		Type_: "Zone",
		// API endpoint for this zone
		Url: "/api/v1/servers/localhost/zones/example.com.",
		// Zone kind, one of “Native”, “Master”, “Slave”
		Kind: "Native",
		// RRSets in this zone
		Rrsets: []pgo.RrSet{},
	}

	ZoneEmpty2 = pgo.Zone{
		// Opaque zone id (string), assigned by the server, should not be interpreted by the application. Guaranteed to be safe for embedding in URLs.
		Id: "mock.test.",
		// Name of the zone (e.g. “mock.test.”) MUST have a trailing dot
		Name: "mock.test.",
		// Set to “Zone”
		Type_: "Zone",
		// API endpoint for this zone
		Url: "/api/v1/servers/localhost/zones/mock.test.",
		// Zone kind, one of “Native”, “Master”, “Slave”
		Kind: "Native",
		// RRSets in this zone
		Rrsets: []pgo.RrSet{},
	}

	ZoneSimple = pgo.Zone{
		// Opaque zone id (string), assigned by the server, should not be interpreted by the application. Guaranteed to be safe for embedding in URLs.
		Id: "example.com.",
		// Name of the zone (e.g. “example.com.”) MUST have a trailing dot
		Name: "example.com.",
		// Set to “Zone”
		Type_: "Zone",
		// API endpoint for this zone
		Url: "/api/v1/servers/localhost/zones/example.com.",
		// Zone kind, one of “Native”, “Master”, “Slave”
		Kind: "Native",
		// RRSets in this zone
		Rrsets: []pgo.RrSet{RRSetSimpleARecord, RRSetSimpleTXTRecord},
	}

	ZonePartial = pgo.Zone{
		// Opaque zone id (string), assigned by the server, should not be interpreted by the application. Guaranteed to be safe for embedding in URLs.
		Id: "example.com.",
		// Name of the zone (e.g. “example.com.”) MUST have a trailing dot
		Name: "example.com.",
		// Set to “Zone”
		Type_: "Zone",
		// API endpoint for this zone
		Url: "/api/v1/servers/localhost/zones/example.com.",
		// Zone kind, one of “Native”, “Master”, “Slave”
		Kind: "Native",
		// RRSets in this zone
		Rrsets: []pgo.RrSet{RRSetSimpleARecord, RRSetSimpleTXTRecord},
	}

	ZoneMixed = pgo.Zone{
		// Opaque zone id (string), assigned by the server, should not be interpreted by the application. Guaranteed to be safe for embedding in URLs.
		Id: "example.com.",
		// Name of the zone (e.g. “example.com.”) MUST have a trailing dot
		Name: "example.com.",
		// Set to “Zone”
		Type_: "Zone",
		// API endpoint for this zone
		Url: "/api/v1/servers/localhost/zones/example.com.",
		// Zone kind, one of “Native”, “Master”, “Slave”
		Kind: "Native",
		// RRSets in this zone
		Rrsets: []pgo.RrSet{RRSetCNAMERecord, RRSetTXTRecord, RRSetMultipleRecords},
	}

	ZoneEmptyToSimplePatch = pgo.Zone{
		// Opaque zone id (string), assigned by the server, should not be interpreted by the application. Guaranteed to be safe for embedding in URLs.
		Id: "example.com.",
		// Name of the zone (e.g. “example.com.”) MUST have a trailing dot
		Name: "example.com.",
		// Set to “Zone”
		Type_: "Zone",
		// API endpoint for this zone
		Url: "/api/v1/servers/localhost/zones/example.com.",
		// Zone kind, one of “Native”, “Master”, “Slave”
		Kind: "Native",
		// RRSets in this zone
		Rrsets: []pgo.RrSet{
			pgo.RrSet{
				Name:       "example.com.",
				Type_:      "A",
				Ttl:        300,
				Changetype: "PATCH",
				Records: []pgo.Record{
					pgo.Record{
						Content:  "8.8.8.8",
						Disabled: false,
						SetPtr:   false,
					},
				},
				Comments: []pgo.Comment(nil),
			},
			pgo.RrSet{
				Name:       "example.com.",
				Type_:      "TXT",
				Ttl:        300,
				Changetype: "PATCH",
				Records: []pgo.Record{
					pgo.Record{
						Content:  "\"heritage=external-dns,external-dns/owner=tower-pdns\"",
						Disabled: false,
						SetPtr:   false,
					},
				},
				Comments: []pgo.Comment(nil),
			},
		},
	}
	ZoneEmptyToSimplePatch2 = pgo.Zone{
		// Opaque zone id (string), assigned by the server, should not be interpreted by the application. Guaranteed to be safe for embedding in URLs.
		Id: "mock.test.",
		// Name of the zone (e.g. “mock.test.”) MUST have a trailing dot
		Name: "mock.test.",
		// Set to “Zone”
		Type_: "Zone",
		// API endpoint for this zone
		Url: "/api/v1/servers/localhost/zones/mock.test.",
		// Zone kind, one of “Native”, “Master”, “Slave”
		Kind: "Native",
		// RRSets in this zone
		Rrsets: []pgo.RrSet{
			pgo.RrSet{
				Name:       "mock.test.",
				Type_:      "A",
				Ttl:        300,
				Changetype: "PATCH",
				Records: []pgo.Record{
					pgo.Record{
						Content:  "9.9.9.9",
						Disabled: false,
						SetPtr:   false,
					},
				},
				Comments: []pgo.Comment(nil),
			},
			pgo.RrSet{
				Name:       "mock.test.",
				Type_:      "TXT",
				Ttl:        300,
				Changetype: "PATCH",
				Records: []pgo.Record{
					pgo.Record{
						Content:  "\"heritage=external-dns,external-dns/owner=tower-pdns\"",
						Disabled: false,
						SetPtr:   false,
					},
				},
				Comments: []pgo.Comment(nil),
			},
		},
	}
	ZoneEmptyToSimpleDelete = pgo.Zone{
		// Opaque zone id (string), assigned by the server, should not be interpreted by the application. Guaranteed to be safe for embedding in URLs.
		Id: "example.com.",
		// Name of the zone (e.g. “example.com.”) MUST have a trailing dot
		Name: "example.com.",
		// Set to “Zone”
		Type_: "Zone",
		// API endpoint for this zone
		Url: "/api/v1/servers/localhost/zones/example.com.",
		// Zone kind, one of “Native”, “Master”, “Slave”
		Kind: "Native",
		// RRSets in this zone
		Rrsets: []pgo.RrSet{
			pgo.RrSet{
				Name:       "example.com.",
				Type_:      "A",
				Ttl:        300,
				Changetype: "DELETE",
				Records: []pgo.Record{
					pgo.Record{
						Content:  "8.8.8.8",
						Disabled: false,
						SetPtr:   false,
					},
				},
				Comments: []pgo.Comment(nil),
			},
			pgo.RrSet{
				Name:       "example.com.",
				Type_:      "TXT",
				Ttl:        300,
				Changetype: "DELETE",
				Records: []pgo.Record{
					pgo.Record{
						Content:  "\"heritage=external-dns,external-dns/owner=tower-pdns\"",
						Disabled: false,
						SetPtr:   false,
					},
				},
				Comments: []pgo.Comment(nil),
			},
		},
	}
)

// API that returns a zone with multiple record types
type PDNSAPIClientStub struct {
}

func (c *PDNSAPIClientStub) ListZones() ([]pgo.Zone, *http.Response, error) {
	return []pgo.Zone{ZoneMixed}, nil, nil
}
func (c *PDNSAPIClientStub) ListZone(zoneId string) (pgo.Zone, *http.Response, error) {
	return ZoneMixed, nil, nil
}
func (c *PDNSAPIClientStub) PatchZone(zoneId string, zoneStruct pgo.Zone) (*http.Response, error) {
	return nil, nil
}

// API that returns a zone with no records
type PDNSAPIClientStubEmptyZones struct {
	// Keep track of all zones we recieve via PatchZone
	patchedZones []pgo.Zone
}

func (c *PDNSAPIClientStubEmptyZones) ListZones() ([]pgo.Zone, *http.Response, error) {
	return []pgo.Zone{ZoneEmpty, ZoneEmpty2}, nil, nil
}
func (c *PDNSAPIClientStubEmptyZones) ListZone(zoneId string) (pgo.Zone, *http.Response, error) {

	if strings.Contains(zoneId, "example.com") {
		return ZoneEmpty, nil, nil
	} else if strings.Contains(zoneId, "mock.test") {
		return ZoneEmpty2, nil, nil
	} else {
		return pgo.Zone{}, nil, nil
	}

}
func (c *PDNSAPIClientStubEmptyZones) PatchZone(zoneId string, zoneStruct pgo.Zone) (*http.Response, error) {
	c.patchedZones = append(c.patchedZones, zoneStruct)
	return nil, nil
}

// API that returns a zone with no records
type PDNSAPIClientStubPatchZoneFailure struct {
	// Anonymous struct for composition
	PDNSAPIClientStubEmptyZones
}

// Just overwrite the PatchZone method to introduce a failure
func (c *PDNSAPIClientStubPatchZoneFailure) PatchZone(zoneId string, zoneStruct pgo.Zone) (*http.Response, error) {
	return nil, errors.New("Generic PDNS Error")
}

// API that returns a zone with no records
type PDNSAPIClientStubListZoneFailure struct {
	// Anonymous struct for composition
	PDNSAPIClientStubEmptyZones
}

// Just overwrite the ListZone method to introduce a failure
func (c *PDNSAPIClientStubListZoneFailure) ListZone(zoneId string) (pgo.Zone, *http.Response, error) {
	return pgo.Zone{}, nil, errors.New("Generic PDNS Error")

}

type PDNSAPIClientStubListZonesFailure struct {
	// Anonymous struct for composition
	PDNSAPIClientStubEmptyZones
}

// Just overwrite the ListZones method to introduce a failure
func (c *PDNSAPIClientStubListZonesFailure) ListZones() ([]pgo.Zone, *http.Response, error) {
	return []pgo.Zone{}, nil, errors.New("Generic PDNS Error")
}

type NewPDNSProviderTestSuite struct {
	suite.Suite
}

func (suite *NewPDNSProviderTestSuite) TestPDNSProviderCreate() {
	// Function definition: NewPDNSProvider(server string, apikey string, domainFilter DomainFilter, dryRun bool) (*PDNSProvider, error)

	_, err := NewPDNSProvider("http://localhost:8081", "", NewDomainFilter([]string{""}), false)
	assert.Error(suite.T(), err, "--pdns-api-key should be specified")

	_, err = NewPDNSProvider("http://localhost:8081", "foo", NewDomainFilter([]string{"example.com", "example.org"}), false)
	assert.Error(suite.T(), err, "--domainfilter should raise an error")

	_, err = NewPDNSProvider("http://localhost:8081", "foo", NewDomainFilter([]string{""}), true)
	assert.Error(suite.T(), err, "--dry-run should raise an error")

	// This is our "regular" code path, no error should be thrown
	_, err = NewPDNSProvider("http://localhost:8081", "foo", NewDomainFilter([]string{""}), false)
	assert.Nil(suite.T(), err, "Regular case should raise no error")
}

func (suite *NewPDNSProviderTestSuite) TestPDNSRRSetToEndpoints() {
	// Function definition: convertRRSetToEndpoints(rr pgo.RrSet) (endpoints []*endpoint.Endpoint, _ error)

	// Create a new provider to run tests against
	p := &PDNSProvider{
		client: &PDNSAPIClientStub{},
	}

	/* given an RRSet with three records, we test:
	   - We correctly create corresponding endpoints
	*/
	eps, err := p.convertRRSetToEndpoints(RRSetMultipleRecords)
	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), endpointsMultipleRecords, eps)

	/* Given an RRSet with two records, one of which is disabled, we test:
	   - We can correctly convert the RRSet into a list of valid endpoints
	   - We correctly discard/ignore the disabled record.
	*/
	eps, err = p.convertRRSetToEndpoints(RRSetDisabledRecord)
	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), endpointsDisabledRecord, eps)

}

func (suite *NewPDNSProviderTestSuite) TestPDNSRecords() {
	// Function definition: Records() (endpoints []*endpoint.Endpoint, _ error)

	// Create a new provider to run tests against
	p := &PDNSProvider{
		client: &PDNSAPIClientStub{},
	}

	/* We test that endpoints are returned correctly for a Zone when Records() is called
	 */
	eps, err := p.Records()
	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), endpointsMixedRecords, eps)

	// Test failures are handled correctly
	// Create a new provider to run tests against
	p = &PDNSProvider{
		client: &PDNSAPIClientStubListZoneFailure{},
	}
	eps, err = p.Records()
	assert.NotNil(suite.T(), err)

	p = &PDNSProvider{
		client: &PDNSAPIClientStubListZonesFailure{},
	}
	eps, err = p.Records()
	assert.NotNil(suite.T(), err)

}

func (suite *NewPDNSProviderTestSuite) TestPDNSConvertEndpointsToZones() {
	// Function definition: ConvertEndpointsToZones(endpoints []*endpoint.Endpoint, changetype pdnsChangeType) (zonelist []pgo.Zone, _ error)

	// Create a new provider to run tests against
	p := &PDNSProvider{
		client: &PDNSAPIClientStubEmptyZones{},
	}

	// Check inserting endpoints from a single zone
	zlist, err := p.ConvertEndpointsToZones(endpointsSimpleRecord, pdnsChangeType("PATCH"))
	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), []pgo.Zone{ZoneEmptyToSimplePatch}, zlist)

	// Check deleting endpoints from a single zone
	zlist, err = p.ConvertEndpointsToZones(endpointsSimpleRecord, pdnsChangeType("DELETE"))
	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), []pgo.Zone{ZoneEmptyToSimpleDelete}, zlist)

	// Check endpoints from multiple zones
	zlist, err = p.ConvertEndpointsToZones(endpointsMultipleZones, pdnsChangeType("PATCH"))
	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), []pgo.Zone{ZoneEmptyToSimplePatch, ZoneEmptyToSimplePatch2}, zlist)

	// Check endpoints from a zone that does not exist
	zlist, err = p.ConvertEndpointsToZones(endpointsNonexistantZone, pdnsChangeType("PATCH"))
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), []pgo.Zone{}, zlist)

	// TODO: Add test to check a record that matches multiple zones (one longer than other), is assigned to the right zone

}

func (suite *NewPDNSProviderTestSuite) TestPDNSmutateRecords() {
	// Function definition: mutateRecords(endpoints []*endpoint.Endpoint, changetype pdnsChangeType) error

	// Create a new provider to run tests against
	c := &PDNSAPIClientStubEmptyZones{}
	p := &PDNSProvider{
		client: c,
	}

	// Check inserting endpoints from a single zone
	err := p.mutateRecords(endpointsSimpleRecord, pdnsChangeType("PATCH"))
	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), []pgo.Zone{ZoneEmptyToSimplePatch}, c.patchedZones)

	// Reset the "patchedZones"
	c.patchedZones = []pgo.Zone{}

	// Check deleting endpoints from a single zone
	err = p.mutateRecords(endpointsSimpleRecord, pdnsChangeType("DELETE"))
	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), []pgo.Zone{ZoneEmptyToSimpleDelete}, c.patchedZones)

	// Check we fail correctly when patching fails for whatever reason
	p = &PDNSProvider{
		client: &PDNSAPIClientStubPatchZoneFailure{},
	}
	// Check inserting endpoints from a single zone
	err = p.mutateRecords(endpointsSimpleRecord, pdnsChangeType("PATCH"))
	assert.NotNil(suite.T(), err)

}
func TestNewPDNSProviderTestSuite(t *testing.T) {
	suite.Run(t, new(NewPDNSProviderTestSuite))
}
