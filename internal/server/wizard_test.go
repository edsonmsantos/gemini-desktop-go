package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"geminidesktop-go/internal/store"

	"github.com/edsonmsantos/browser-bridge/crxsign"
	"github.com/edsonmsantos/browser-bridge/policymgr"
)

func TestBrowserWizard_Endpoints_TDD(t *testing.T) {
	tmpDir := t.TempDir()
	st, err := store.NewStore(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("falha ao criar store: %v", err)
	}
	defer st.Close()

	srv := NewServerV2(nil, tmpDir, nil, st)
	fakeWriter := &policymgr.FakePolicyWriter{
		Store: make(map[string]string),
	}
	srv.SetPolicyWriter(fakeWriter)
	srv.SetBrowserBridge(9223, "")
	srv.SetBrowserBridgeUpdateBaseURL("http://127.0.0.1:9223/update")

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 1. GET /api/browser/wizard/state inicial
	resp, err := http.Get(ts.URL + "/api/browser/wizard/state")
	if err != nil {
		t.Fatalf("falha ao consultar /api/browser/wizard/state: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("esperado status 200 em wizard state, obteve %d", resp.StatusCode)
	}

	var state BrowserWizardStateResponse
	if err := json.NewDecoder(resp.Body).Decode(&state); err != nil {
		t.Fatalf("falha ao decodificar JSON do wizard state: %v", err)
	}

	if len(state.Browsers) == 0 {
		t.Fatalf("esperado lista de navegadores não vazia para o SO %s", runtime.GOOS)
	}
	if state.UpdateBaseURL != "http://127.0.0.1:9223/update" {
		t.Errorf("esperado updateBaseURL http://127.0.0.1:9223/update, obteve %s", state.UpdateBaseURL)
	}

	// Nenhum browser deve ter política aplicada inicialmente
	targetBrowser := state.Browsers[0].Browser
	for _, b := range state.Browsers {
		if b.PolicyApplied {
			t.Errorf("esperado policyApplied=false inicialmente para %s", b.Browser)
		}
	}

	// 2. POST /api/browser/install-policy com updateUrl customizada
	customURL := "https://meu-servidor.local/bb/update"
	installBody, _ := json.Marshal(InstallPolicyRequest{
		Browser:   targetBrowser,
		UpdateURL: customURL,
	})
	respInstall, err := http.Post(ts.URL+"/api/browser/install-policy", "application/json", bytes.NewReader(installBody))
	if err != nil {
		t.Fatalf("falha ao chamar install-policy: %v", err)
	}
	defer respInstall.Body.Close()

	if respInstall.StatusCode != http.StatusOK {
		t.Fatalf("esperado status 200 em install-policy, obteve %d", respInstall.StatusCode)
	}

	var installRes InstallPolicyResponse
	if err := json.NewDecoder(respInstall.Body).Decode(&installRes); err != nil {
		t.Fatalf("falha ao decodificar resposta install-policy: %v", err)
	}
	if !installRes.OK {
		t.Errorf("esperado ok=true em install-policy")
	}
	if !strings.Contains(installRes.NextSteps, "reinicie") {
		t.Errorf("esperado instrução de reinício no nextSteps, obteve %s", installRes.NextSteps)
	}

	// Verifica se a política foi salva no fakeWriter com o ID da extensão e URL corretos
	targets := policymgr.GetPolicyTargets(runtime.GOOS, targetBrowser)
	if len(targets) == 0 {
		t.Fatalf("nenhum target encontrado para %s", targetBrowser)
	}
	applied, content, _ := fakeWriter.CheckPolicy(targets[0])
	if !applied {
		t.Fatalf("esperado que a política tenha sido aplicada no FakePolicyWriter")
	}
	if !strings.Contains(content, crxsign.ExtensionID()) {
		t.Errorf("conteúdo da política não contém o ID da extensão (%s)", crxsign.ExtensionID())
	}
	if !strings.Contains(content, customURL) {
		t.Errorf("conteúdo da política não contém a URL customizada (%s)", customURL)
	}

	// 3. GET /api/browser/wizard/state novamente para confirmar que agora está aplicado
	resp2, err := http.Get(ts.URL + "/api/browser/wizard/state")
	if err != nil {
		t.Fatalf("falha no segundo get wizard state: %v", err)
	}
	defer resp2.Body.Close()

	var state2 BrowserWizardStateResponse
	_ = json.NewDecoder(resp2.Body).Decode(&state2)
	foundApplied := false
	for _, b := range state2.Browsers {
		if b.Browser == targetBrowser && b.PolicyApplied {
			foundApplied = true
			break
		}
	}
	if !foundApplied {
		t.Errorf("esperado que %s agora tenha policyApplied=true", targetBrowser)
	}

	// 4. POST /api/browser/remove-policy
	removeBody, _ := json.Marshal(RemovePolicyRequest{
		Browser: targetBrowser,
	})
	respRemove, err := http.Post(ts.URL+"/api/browser/remove-policy", "application/json", bytes.NewReader(removeBody))
	if err != nil {
		t.Fatalf("falha ao chamar remove-policy: %v", err)
	}
	defer respRemove.Body.Close()

	if respRemove.StatusCode != http.StatusOK {
		t.Fatalf("esperado status 200 em remove-policy, obteve %d", respRemove.StatusCode)
	}

	// Confirma que foi removido do fakeWriter
	appliedAfterRemove, _, _ := fakeWriter.CheckPolicy(targets[0])
	if appliedAfterRemove {
		t.Errorf("esperado que política tenha sido removida do FakePolicyWriter")
	}
}
