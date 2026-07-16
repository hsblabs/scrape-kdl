package executor

import (
	"context"
	"encoding/json"
	"os"
	"testing"
)

func TestCancellationCodeMatchesSharedRuntimeContract(t *testing.T) {
	data, err := os.ReadFile("../../testdata/runtime-contract/cancellation.json")
	if err != nil {
		t.Fatal(err)
	}
	var contract struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(data, &contract); err != nil {
		t.Fatal(err)
	}
	if got := operationErrorCode("E_OPERATION", context.Canceled); got != contract.Code {
		t.Fatalf("cancellation code = %q, shared contract = %q", got, contract.Code)
	}
}
