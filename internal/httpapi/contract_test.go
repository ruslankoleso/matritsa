package httpapi

import (
	"context"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func loadSpec(t *testing.T) *openapi3.T {
	t.Helper()
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile("../../api/openapi.yaml")
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}
	return doc
}

func TestOpenAPI_Valid(t *testing.T) {
	doc := loadSpec(t)
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatalf("spec invalid: %v", err)
	}
	seen := map[string]string{}
	for path, item := range doc.Paths.Map() {
		for method, op := range item.Operations() {
			if op.OperationID == "" {
				t.Errorf("%s %s has no operationId", method, path)
			}
			if prev, dup := seen[op.OperationID]; dup {
				t.Errorf("operationId %s used by %s and %s %s", op.OperationID, prev, method, path)
			}
			seen[op.OperationID] = method + " " + path
		}
	}
	for _, id := range []string{"login", "refreshToken", "getMe", "listProducts", "lookupCompany", "recommendProduct",
		"listApplications", "createApplication", "getApplication", "updateApplication", "submitApplication", "uploadDocument",
		"respondToInfoRequest", "acceptOffer", "declineOffer", "cancelApplication", "getTimeline", "getFeed",
		"listNotifications", "markNotificationRead", "streamEvents", "getQueue", "getStaffApplication", "claimApplication",
		"makeDecision", "getExplanation", "register", "lookupCompanyForSignup", "registrationExamples"} {
		if _, ok := seen[id]; !ok {
			t.Errorf("operation %s missing from the contract", id)
		}
	}
}

func TestOpenAPI_PublicOperationsAreExplicit(t *testing.T) {
	doc := loadSpec(t)
	public := map[string]bool{"login": true, "refreshToken": true, "listProducts": true,
		"register": true, "lookupCompanyForSignup": true, "registrationExamples": true}
	for _, item := range doc.Paths.Map() {
		for _, op := range item.Operations() {
			explicitlyOpen := op.Security != nil && len(*op.Security) == 0
			if public[op.OperationID] && !explicitlyOpen {
				t.Errorf("%s must declare security: []", op.OperationID)
			}
			if !public[op.OperationID] && explicitlyOpen {
				t.Errorf("%s is unexpectedly public", op.OperationID)
			}
		}
	}
}

func TestOpenAPI_StaffPathsAreUnderStaff(t *testing.T) {
	doc := loadSpec(t)
	for path, item := range doc.Paths.Map() {
		for _, op := range item.Operations() {
			isStaffTag := len(op.Tags) > 0 && op.Tags[0] == "staff"
			isStaffPath := len(path) >= 6 && path[:6] == "/staff"
			if isStaffTag != isStaffPath {
				t.Errorf("%s: tag/path mismatch (staff tag=%v, staff path=%v)", path, isStaffTag, isStaffPath)
			}
		}
	}
}
