package provider

import (
	//"context"
	//"errors"
	"net/http"
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

	//
	//        	// This struct contains a lot more fields, but we only set the ones we care about
	//        	ZoneExampleDotCom = pgo.Zone{
	//        		// Opaque zone id (string), assigned by the server, should not be interpreted by the application. Guaranteed to be safe for embedding in URLs.
	//        		Id: "example.com",
	//        		// Name of the zone (e.g. “example.com.”) MUST have a trailing dot
	//        		Name: "example.com",
	//        		// Set to “Zone”
	//        		Type_: "Zone",
	//        		// API endpoint for this zone
	//        		Url: "/api/v1/servers/localhost/zones/example.com.",
	//        		// Zone kind, one of “Native”, “Master”, “Slave”
	//        		Kind: "Native",
	//        		// RRSets in this zone
	//        		//Rrsets []RrSet `json:"rrsets,omitempty"`
	//        	}
	//
	ZoneSimple = pgo.Zone{
		// Opaque zone id (string), assigned by the server, should not be interpreted by the application. Guaranteed to be safe for embedding in URLs.
		Id: "example.com",
		// Name of the zone (e.g. “example.com.”) MUST have a trailing dot
		Name: "example.com",
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
		Id: "example.com",
		// Name of the zone (e.g. “example.com.”) MUST have a trailing dot
		Name: "example.com",
		// Set to “Zone”
		Type_: "Zone",
		// API endpoint for this zone
		Url: "/api/v1/servers/localhost/zones/example.com.",
		// Zone kind, one of “Native”, “Master”, “Slave”
		Kind: "Native",
		// RRSets in this zone
		Rrsets: []pgo.RrSet{RRSetCNAMERecord, RRSetTXTRecord, RRSetMultipleRecords},
	}
)

//
//        /*
//        type mockApiClient struct {
//        	ZonesApi ZonesApiInterface
//        }
//        */
//        type mockApiClient pgo.APIClient
//
//        /*
//        type ZonesApiInterface interface {
//        	PatchZone(ctx context.Context, serverId string, zoneId string, zoneStruct pgo.Zone) (*http.Response, error)
//        	ListZone(ctx context.Context, serverId string, zoneId string) (pgo.Zone, *http.Response, error)
//        	ListZones(ctx context.Context, serverId string) ([]pgo.Zone, *http.Response, error)
//        }
//        */
//
//        type ZonesApiSuccess pgo.ZonesApiService
//        type ZonesApiFail pgo.ZonesApiService
//
//        func (a *ZonesApiSuccess) PatchZone(ctx context.Context, serverId string, zoneId string, zoneStruct pgo.Zone) (*http.Response, error) {
//        	return nil, nil
//        }
//        func (a *ZonesApiSuccess) ListZone(ctx context.Context, serverId string, zoneId string) (pgo.Zone, *http.Response, error) {
//        	return ZoneSimple, nil, nil
//        }
//        func (a *ZonesApiSuccess) ListZones(ctx context.Context, serverId string) ([]pgo.Zone, *http.Response, error) {
//        	return []pgo.Zone{ZoneSimple}, nil, nil
//        }
//
//        func (a *ZonesApiFail) PatchZone(ctx context.Context, serverId string, zoneId string, zoneStruct pgo.Zone) (*http.Response, error) {
//        	return nil, errors.New("Patching zone failed")
//        }
//        func (a *ZonesApiFail) ListZone(ctx context.Context, serverId string, zoneId string) (pgo.Zone, *http.Response, error) {
//        	return pgo.Zone{}, nil, errors.New("Listing Zone failed")
//        }
//        func (a *ZonesApiFail) ListZones(ctx context.Context, serverId string) ([]pgo.Zone, *http.Response, error) {
//        	return nil, nil, errors.New("Failed to retrieve zones")
//        }
//

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

// Function accepts an implementation of the APIProvider Interface and returns
// a PDNS provider. Useful for creating external-dns providers with the PDNS
// API client mocked out
/*
func NewMockPDNSProvider(stub PDNSAPIProvider{}) (*PDNSProvider, error) {
	provider := &PDNSProvider{
		client: &stub{},
	}

	return provider, nil
}
*/

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
	// Function definition: convertRRSetToEndpoints(rr pgo.RrSet) (endpoints []*endpoint.Endpoint, _ error)

	// Create a new provider to run tests against
	p := &PDNSProvider{
		client: &PDNSAPIClientStub{},
	}

	/* We test that endpoints are returned correctly for a Zone when Records() is called
	 */
	eps, err := p.Records()
	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), endpointsMixedRecords, eps)

}

//
//        func (suite *NewPDNSProviderTestSuite) TestPDNSEndpointsToZone() {
//        	// function definition: (p *PDNSProvider) ConvertEndpointsToZones(endpoints []*endpoint.Endpoint, changetype pdnsChangeType) (zonelist []pgo.Zone, _ error)
//
//        	// Create a new provider to run tests against
//        	_, err := NewPDNSProvider("http://localhost:8081", "foo", NewDomainFilter([]string{""}), false)
//        	// Make sure we can create the provider correctly
//        	assert.Nil(suite.T(), err)
//        }
func TestNewPDNSProviderTestSuite(t *testing.T) {
	suite.Run(t, new(NewPDNSProviderTestSuite))
}

//
//        type MockTestSuite struct {
//        	suite.Suite
//        }
//
//        func (suite *MockTestSuite) TestPDNSZones() {
//        	// There is literally nothing here to test, it's just a shim around the api zones call that catches errors.
//
//        	client := (*pgo.APIClient)(&mockApiClient{})
//        	client.ZonesApi = (*pgo.ZonesApiService)(&ZonesApiSuccess{})
//        	provider := PDNSProvider{
//        		// Swagger API Client
//        		client: client,
//        	}
//
//        	z, err := provider.Zones()
//        	assert.Nil(suite.T(), err)
//        	assert.Equal(suite.T(), z, []pgo.Zone{ZoneSimple})
//
//        }
//        func (suite *MockTestSuite) TestPDNSRecords() {
//        }
//        func (suite *MockTestSuite) TestPDNSApplyChanges() {
//        }
//        func (suite *MockTestSuite) TestPDNSmutateRecords() {
//        }
//        func TestPDNSUnitTestSuite(t *testing.T) {
//        	suite.Run(t, new(MockTestSuite))
//        }
