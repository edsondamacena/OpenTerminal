// Command seed-hosts popula um cofre de desenvolvimento com hosts sintéticos.
package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"openterminal/internal/domain"
	"openterminal/internal/model"
	"openterminal/internal/vault"
)

var topGroups = []string{"Produção", "Homologação", "Clientes", "Laboratório", "Legado"}
var subgroups = []string{"Web", "Banco", "Filas", "Monitoramento"}

func run(dataDir, password string, count int) error {
	return runWithKDF(dataDir, password, count, vault.DefaultKDF)
}

func runWithKDF(dataDir, password string, count int, kdf func([]byte) vault.KDFParams) error {
	store, err := vault.Open(dataDir, kdf)
	if err != nil {
		return err
	}
	defer store.Close()

	switch store.Status().State {
	case vault.StateNonexistent:
		if err := store.Create(password, password); err != nil {
			return err
		}
	case vault.StateLocked:
		if err := store.Unlock(password); err != nil {
			return err
		}
	default:
		return fmt.Errorf("estado do cofre incompatível: %s", store.Status().State)
	}

	now := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	if err := store.Mutate(func(content *model.Content) error {
		leaves := make([]string, 0, len(topGroups)*(len(subgroups)+1))
		for _, name := range topGroups {
			top, err := domain.CreateGroup(content, name, nil)
			if err != nil {
				return err
			}
			for _, subgroup := range subgroups {
				child, err := domain.CreateGroup(content, subgroup, model.Ptr(top.ID))
				if err != nil {
					return err
				}
				leaves = append(leaves, child.ID)
			}
			leaves = append(leaves, top.ID)
		}
		for i := 0; i < count; i++ {
			var groupID *string
			if i%10 != 0 {
				groupID = model.Ptr(leaves[i%len(leaves)])
			}
			usernames := [...]string{"deploy", "admin", "ops"}
			_, err := domain.CreateHost(content, domain.HostInput{
				Label:      fmt.Sprintf("srv-%05d", i+1),
				Address:    fmt.Sprintf("10.%d.%d.%d", (i>>16)&255, (i>>8)&255, i&255),
				Port:       22,
				Username:   usernames[i%len(usernames)],
				AuthMethod: model.AuthAsk,
				GroupID:    groupID,
			}, now)
			if err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	store.Lock()
	return nil
}

func main() {
	dataDir := os.Getenv("OPENTERMINAL_DATA_DIR")
	password := os.Getenv("OPENTERMINAL_DEV_PASSWORD")
	if dataDir == "" {
		fmt.Fprintln(os.Stderr, "Defina OPENTERMINAL_DATA_DIR com uma pasta de desenvolvimento.")
		os.Exit(1)
	}
	if password == "" {
		fmt.Fprintln(os.Stderr, "Defina OPENTERMINAL_DEV_PASSWORD com a senha mestra do cofre de desenvolvimento.")
		os.Exit(1)
	}
	count := 1000
	if len(os.Args) > 1 {
		var err error
		count, err = strconv.Atoi(os.Args[1])
		if err != nil {
			count = 0
		}
	}
	if count < 1 || count > 100_000 {
		fmt.Fprintln(os.Stderr, "Quantidade inválida (1–100000).")
		os.Exit(1)
	}
	if err := run(dataDir, password, count); err != nil {
		fmt.Fprintln(os.Stderr, "falha ao popular o cofre:", err)
		os.Exit(1)
	}
	fmt.Printf("%d hosts criados em %s\n", count, dataDir)
}
