package lime

// Common test constants used across multiple test files
const (
	// Domain constants
	testDomain        = "limeprotocol.org"
	testExampleDomain = "example.com"

	// ID constants
	testCommandID = "4609d0a3-00eb-4e16-9d44-27d115c6eb31"
	testSessionID = "52e59849-19a8-4b2d-86b7-3fa563cdb616"

	// Instance constants
	testServerInstance = "#server1"
	testClientInstance = "test-instance"

	// MediaType constants
	testAccountMediaType = "vnd.lime.account"

	// Account data constants
	testName    = "John Doe"
	testAddress = "Main street"
	testCity    = "Belo Horizonte"

	// Authentication constants
	testPassword          = "bXl2ZXJ5c2VjcmV0cGFzc3dvcmQ="
	testPasswordPlainText = "correct-password"
	testKey               = "valid-key-123"
	testToken             = "valid-token"
	testTrustedIssuer     = "trusted-issuer"

	// Text content constants
	testHelloWorld = "Hello world"

	// Format strings
	testHelloWorldFormat = "Hello world %v!"

	// URI constants
	testURIPath          = "/test/path"
	testURIPathWithQuery = "/test/path?key=value"
	testURIFull          = "lime://user@example.com/path/to/resource"

	// Common assertion messages
	chainMsg = "should return self for method chaining"
)
