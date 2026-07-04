package workmodel

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const deliverableSchemaLiteral = "p0_p1_file_line"

func TestExpandLegacySchemaToContract(t *testing.T) {
	c := ExpandLegacySchemaToContract(FirstRegisteredDeliverableSchema())
	if !c.ContractApplicable() {
		t.Fatal("expected applicable contract from legacy schema")
	}
	if c.Citation != DeliverableCitationFileLine || c.Severity != DeliverableSeverityP0P1 {
		t.Fatalf("contract = %+v", c)
	}
}

func TestVerifyDeliverableContract_ReviewContract(t *testing.T) {
	c := DefaultTestDeliverableContract()
	got := VerifyDeliverableContract(c, "P0: nil deref in internal/foo/bar.go:42", "final_answer")
	if got.Status != DeliverableStatusComplete {
		t.Fatalf("status = %q, want complete", got.Status)
	}
	if got.Payload == nil || len(got.Payload.Findings) == 0 {
		t.Fatal("expected parsed findings payload")
	}
}

func TestNarrowestContract(t *testing.T) {
	wide := DeliverableContract{Citation: DeliverableCitationFileLine}
	narrow := DefaultTestDeliverableContract()
	got := NarrowestContract(wide, narrow)
	if got.Severity != DeliverableSeverityP0P1 {
		t.Fatalf("severity = %q", got.Severity)
	}
}

func TestEffectiveExecuteMaxIters(t *testing.T) {
	jsonContract := DeliverableContract{Structure: DeliverableStructureFindingsJSON}
	if got := EffectiveExecuteMaxIters(3, 5, jsonContract); got != DefaultFindingsJSONExecuteIters {
		t.Fatalf("findings_json floor = %d, want %d", got, DefaultFindingsJSONExecuteIters)
	}
	free := DeliverableContract{Structure: DeliverableStructureFreeText}
	if got := EffectiveExecuteMaxIters(3, 5, free); got != 3 {
		t.Fatalf("free text = %d, want 3", got)
	}
}

func TestNoStrayDeliverableSchemaLiterals(t *testing.T) {
	root, err := findRepoRootForContractTest()
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	allowed := map[string]struct{}{
		filepath.Join(root, "internal/layers/orchestration/workmodel/deliverable_contract.go"):         {},
		filepath.Join(root, "internal/layers/orchestration/workmodel/deliverable_contract_test.go"):     {},
		filepath.Join(root, "internal/layers/contextengine/materialize/format_hints_test.go"):           {},
	}
	scanRoots := []string{
		filepath.Join(root, "internal/layers/orchestration"),
		filepath.Join(root, "internal/layers/contextengine/i18n"),
		filepath.Join(root, "internal/layers/contextengine/materialize"),
	}
	for _, scanRoot := range scanRoots {
		err := filepath.WalkDir(scanRoot, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".go") {
				return nil
			}
			if _, ok := allowed[path]; ok {
				return nil
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if strings.Contains(string(raw), deliverableSchemaLiteral) {
				t.Errorf("%s contains stray deliverable schema literal %q; use DeliverableContract dimensions", path, deliverableSchemaLiteral)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", scanRoot, err)
		}
	}
}

func findRepoRootForContractTest() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for i := 0; i < 16; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", os.ErrNotExist
}
