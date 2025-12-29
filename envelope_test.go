package lime

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnvelopeSetID(t *testing.T) {
	env := &Envelope{}
	result := env.SetID("test-id-123")
	assert.Equal(t, "test-id-123", env.ID)
	assert.Equal(t, env, result, "should return self for method chaining")
}

func TestEnvelopeSetNewEnvelopeID(t *testing.T) {
	env := &Envelope{}
	result := env.SetNewEnvelopeID()
	assert.NotEmpty(t, env.ID, "should generate a new ID")
	assert.Equal(t, env, result, "should return self for method chaining")
}

func TestEnvelopeSetFrom(t *testing.T) {
	env := &Envelope{}
	from := Node{
		Identity: Identity{Name: "user", Domain: "example.com"},
		Instance: "home",
	}
	result := env.SetFrom(from)
	assert.Equal(t, from, env.From)
	assert.Equal(t, env, result, "should return self for method chaining")
}

func TestEnvelopeSetFromString(t *testing.T) {
	env := &Envelope{}
	result := env.SetFromString("user@example.com/home")
	assert.Equal(t, "user", env.From.Identity.Name)
	assert.Equal(t, "example.com", env.From.Identity.Domain)
	assert.Equal(t, "home", env.From.Instance)
	assert.Equal(t, env, result, "should return self for method chaining")
}

func TestEnvelopeSetTo(t *testing.T) {
	env := &Envelope{}
	to := Node{
		Identity: Identity{Name: "receiver", Domain: "domain.com"},
		Instance: "work",
	}
	result := env.SetTo(to)
	assert.Equal(t, to, env.To)
	assert.Equal(t, env, result, "should return self for method chaining")
}

func TestEnvelopeSetToString(t *testing.T) {
	env := &Envelope{}
	result := env.SetToString("receiver@domain.com/work")
	assert.Equal(t, "receiver", env.To.Identity.Name)
	assert.Equal(t, "domain.com", env.To.Identity.Domain)
	assert.Equal(t, "work", env.To.Instance)
	assert.Equal(t, env, result, "should return self for method chaining")
}

func TestEnvelopeSetPP(t *testing.T) {
	env := &Envelope{}
	pp := Node{
		Identity: Identity{Name: "delegate", Domain: "proxy.com"},
		Instance: "instance1",
	}
	result := env.SetPP(pp)
	assert.Equal(t, pp, env.PP)
	assert.Equal(t, env, result, "should return self for method chaining")
}

func TestEnvelopeSetPPString(t *testing.T) {
	env := &Envelope{}
	result := env.SetPPString("delegate@proxy.com/instance1")
	assert.Equal(t, "delegate", env.PP.Identity.Name)
	assert.Equal(t, "proxy.com", env.PP.Identity.Domain)
	assert.Equal(t, "instance1", env.PP.Instance)
	assert.Equal(t, env, result, "should return self for method chaining")
}

func TestEnvelopeSetMetadataKeyValue(t *testing.T) {
	env := &Envelope{}
	result := env.SetMetadataKeyValue("key1", "value1")
	assert.Equal(t, "value1", env.Metadata["key1"])
	assert.Equal(t, env, result, "should return self for method chaining")

	// Test adding more metadata
	env.SetMetadataKeyValue("key2", "value2")
	assert.Equal(t, "value1", env.Metadata["key1"])
	assert.Equal(t, "value2", env.Metadata["key2"])
}

func TestEnvelopeSetMetadataKeyValueInitializesMap(t *testing.T) {
	env := &Envelope{Metadata: nil}
	env.SetMetadataKeyValue("test", "value")
	assert.NotNil(t, env.Metadata, "should initialize metadata map if nil")
	assert.Equal(t, "value", env.Metadata["test"])
}

func TestEnvelopeMethodChaining(t *testing.T) {
	env := &Envelope{}
	result := env.
		SetID("chain-id").
		SetFromString("sender@example.com").
		SetToString("receiver@example.com").
		SetPPString("delegate@proxy.com").
		SetMetadataKeyValue("key", "value")

	assert.Equal(t, "chain-id", env.ID)
	assert.Equal(t, "sender", env.From.Identity.Name)
	assert.Equal(t, "receiver", env.To.Identity.Name)
	assert.Equal(t, "delegate", env.PP.Identity.Name)
	assert.Equal(t, "value", env.Metadata["key"])
	assert.Equal(t, env, result, "should support method chaining")
}

func TestNewEnvelopeID(t *testing.T) {
	id1 := NewEnvelopeID()
	id2 := NewEnvelopeID()

	assert.NotEmpty(t, id1, "should generate non-empty ID")
	assert.NotEmpty(t, id2, "should generate non-empty ID")
	assert.NotEqual(t, id1, id2, "should generate unique IDs")
}
func TestReasonString(t *testing.T) {
	reason := Reason{
		Code:        404,
		Description: "Resource not found",
	}

	str := reason.String()
	assert.Contains(t, str, "404", "should contain code")
	assert.Contains(t, str, "Resource not found", "should contain description")
	assert.Contains(t, str, "Code:", "should contain code label")
	assert.Contains(t, str, "Description:", "should contain description label")
}

func TestReasonStringEmpty(t *testing.T) {
	reason := Reason{}
	str := reason.String()
	assert.Contains(t, str, "Code:", "should contain code label")
	assert.Contains(t, str, "Description:", "should contain description label")
}
