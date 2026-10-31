package bridge

import (
	"reflect"
	"slices"
	"testing"
)

// Métodos de cada serviço ligado ao Wails, conforme specs/002-wails-migration/contracts/bridge.md.
// Qualquer método exportado a mais vira função chamável pela interface, então a lista é exata.
var contract = map[string][]string{
	"VaultService": {"GetStatus", "Create", "Unlock", "Lock", "ChangePassword", "Erase",
		"GetSettings", "UpdateSettings", "ReportActivity"},
	"HostService": {"List", "Create", "Update", "Duplicate", "Remove", "Move",
		"ListGroups", "CreateGroup", "RenameGroup", "MoveGroup", "RemoveGroup", "ImportTabby", "ImportCSV", "ExportBackup", "ImportBackup"},
	"CredentialService": {"List", "CreatePassword", "PickKeyFile", "ImportKey", "GenerateEd25519",
		"CopyPublicKey", "Rename", "Remove"},
	"SessionService": {"OpenSsh", "OpenLocal", "Write", "Resize", "Reconnect", "Close",
		"AnswerHostKey", "AnswerPrompt", "ActiveSummary", "SendKeys"},
	"SftpService": {"Open", "List", "Mkdir", "Rename", "Remove", "Upload", "Download",
		"ResolveConflict", "Cancel"},
	"ForwardService": {"List", "ListActive", "Create", "Update", "Remove", "Activate", "Deactivate"},
	"ProfileService": {"GetGlobal", "UpdateGlobal", "GetForHost", "SetHostOverride", "Effective",
		"ListSchemes", "SaveScheme", "DuplicateScheme", "RemoveScheme", "ImportScheme",
		"ExportScheme", "ListFonts"},
	"AppService": {"GetAppearance", "GetPreferences", "ListLocalShells", "UpdatePreferences", "ConfirmClose",
		"WindowMinimise", "WindowToggleMaximise", "WindowClose", "OpenExternalUrl", "CheckUpdate", "InstallUpdate"},
	"SyncService": {"GetStatus", "Connect", "CancelConnect", "Disconnect", "SyncNow"},
}

func TestBoundServicesMatchContract(t *testing.T) {
	seen := map[string]bool{}
	for _, svc := range (&Services{}).Bound() {
		typ := reflect.TypeOf(svc)
		name := typ.Elem().Name()
		want, ok := contract[name]
		if !ok {
			t.Errorf("serviço %s ligado ao Wails mas fora do contrato", name)
			continue
		}
		seen[name] = true
		var got []string
		for i := range typ.NumMethod() {
			got = append(got, typ.Method(i).Name)
		}
		slices.Sort(got)
		w := slices.Clone(want)
		slices.Sort(w)
		if !slices.Equal(got, w) {
			t.Errorf("%s: métodos %v, contrato %v", name, got, w)
		}
	}
	for name := range contract {
		if !seen[name] {
			if allServicesWired {
				t.Errorf("serviço %s do contrato não está ligado", name)
			} else {
				t.Logf("serviço %s ainda não ligado", name)
			}
		}
	}
}
