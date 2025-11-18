package nvmet

import (
	"fmt"
	"path/filepath"
)

// Referral represents an NVMe Referral in configFS.
//
//nolint:govet // field alignment optimization is not critical for this struct
type Referral struct {
	CFSNode
	port *Port
	name string
}

// NewReferral creates a new Referral instance.
//
//nolint:gocritic // parameter type combination would reduce readability
func NewReferral(port *Port, name string, mode string) (*Referral, error) {
	//nolint:exhaustruct,varnamelen,lll // CFSNode, port, and name are initialized below; r is a standard short variable name for referral
	r := &Referral{}

	if port == nil {
		return nil, &CFSError{Msg: "Invalid parent port"}
	}

	r.CFSNode.path = configfsDir
	r.attrGroups = []string{"addr"}
	r.port = port
	r.name = name
	r.path = filepath.Join(port.path, "referrals", name)

	if err := r.createInCFS(mode); err != nil {
		return nil, err
	}

	return r, nil
}

// Name returns the referral name.
func (r *Referral) Name() string {
	return r.name
}

// Port returns the parent port.
func (r *Referral) Port() *Port {
	return r.port
}

// SetupReferral sets up a Referral based on a map from saved config.
func SetupReferral(port *Port, data map[string]interface{}, errFunc func(string)) error {
	name, ok := data["name"].(string)
	if !ok {
		errFunc("'name' not defined for Referral")

		return nil
	}

	ref, err := NewReferral(port, name, "any")
	if err != nil {
		errFunc(fmt.Sprintf("Could not create Referral object: %v", err))

		return nil
	}

	ref.setupAttrs(data, errFunc)

	return nil
}

// Dump returns a JSON-serializable representation of the referral.
func (r *Referral) Dump() (map[string]interface{}, error) {
	result, err := r.CFSNode.Dump()
	if err != nil {
		return nil, err
	}

	result["name"] = r.name

	return result, nil
}
