package bridge

import (
	"openterminal/internal/apperr"
	"openterminal/internal/domain"
	"openterminal/internal/forwarding"
	"openterminal/internal/model"
	"openterminal/internal/validate"
	"openterminal/internal/vault"
)

type ForwardRuleView struct {
	model.ForwardRule
	forwarding.Status
}

type ForwardService struct {
	store   *vault.Store
	manager *forwarding.Manager
}

func newForwardService(store *vault.Store, manager *forwarding.Manager) *ForwardService {
	return &ForwardService{store: store, manager: manager}
}

func (s *ForwardService) List(hostID string) ([]ForwardRuleView, error) {
	if err := validate.UUID(hostID); err != nil {
		return nil, clean(err)
	}
	content, err := s.store.Read()
	if err != nil {
		return nil, clean(err)
	}
	if _, err := domain.FindHost(content, hostID); err != nil {
		return nil, clean(err)
	}
	rules := domain.ListForwardRulesForHost(content, hostID)
	out := make([]ForwardRuleView, len(rules))
	for i := range rules {
		out[i] = ForwardRuleView{ForwardRule: rules[i], Status: s.manager.Status(rules[i].ID)}
	}
	return out, nil
}

func (s *ForwardService) ListActive() ([]forwarding.ActiveForward, error) {
	return s.manager.ListActive(), nil
}

func (s *ForwardService) Create(input domain.ForwardRuleInput) (model.ForwardRule, error) {
	var created model.ForwardRule
	err := s.store.Mutate(func(content *model.Content) error {
		var err error
		created, err = domain.CreateForwardRule(content, input)
		return err
	})
	return created, clean(err)
}

func (s *ForwardService) Update(id string, input domain.ForwardRuleInput) (model.ForwardRule, error) {
	if err := validate.UUID(id); err != nil {
		return model.ForwardRule{}, clean(err)
	}
	s.manager.Deactivate(id)
	var updated model.ForwardRule
	err := s.store.Mutate(func(content *model.Content) error {
		var err error
		updated, err = domain.UpdateForwardRule(content, id, input)
		return err
	})
	return updated, clean(err)
}

func (s *ForwardService) Remove(id string) error {
	if err := validate.UUID(id); err != nil {
		return clean(err)
	}
	s.manager.Deactivate(id)
	return clean(s.store.Mutate(func(content *model.Content) error {
		return domain.RemoveForwardRule(content, id)
	}))
}

func (s *ForwardService) Activate(id string) error {
	if err := validate.UUID(id); err != nil {
		return clean(err)
	}
	content, err := s.store.Read()
	if err != nil {
		return clean(err)
	}
	rule, err := domain.FindForwardRule(content, id)
	if err != nil {
		return clean(err)
	}
	host, err := domain.FindHost(content, rule.HostID)
	if err != nil {
		return clean(err)
	}
	err = s.manager.Activate(rule, host.Label)
	if err != nil && apperr.CodeOf(err) == apperr.HostKeyRejected {
		s.manager.Deactivate(rule.ID)
	}
	return clean(err)
}

// Deactivate não toca no cofre: encaminhamentos continuam controláveis após o bloqueio.
func (s *ForwardService) Deactivate(id string) error {
	if err := validate.UUID(id); err != nil {
		return clean(err)
	}
	s.manager.Deactivate(id)
	return nil
}
