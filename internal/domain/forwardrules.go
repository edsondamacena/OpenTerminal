package domain

import (
	"net"
	"strings"
	"unicode/utf8"

	"openterminal/internal/apperr"
	"openterminal/internal/model"
	"openterminal/internal/validate"
)

const DefaultBindAddress = "127.0.0.1"

type ForwardRuleInput struct {
	HostID           string  `json:"hostId"`
	Label            *string `json:"label"`
	BindAddress      string  `json:"bindAddress"`
	AllowNonLoopback bool    `json:"allowNonLoopback"`
	BindPort         int     `json:"bindPort"`
	DestHost         string  `json:"destHost"`
	DestPort         int     `json:"destPort"`
}

func IsLoopback(address string) bool {
	if strings.EqualFold(address, "localhost") {
		return true
	}
	ip := net.ParseIP(address)
	return ip != nil && ip.IsLoopback()
}

func normalizeForwardInput(c *model.Content, in ForwardRuleInput, selfID string) (ForwardRuleInput, error) {
	if validate.UUID(in.HostID) != nil {
		return in, invalid()
	}
	if _, err := FindHost(c, in.HostID); err != nil {
		return in, invalid()
	}
	if in.Label != nil {
		label := strings.TrimSpace(*in.Label)
		if label == "" {
			in.Label = nil
		} else if utf8.RuneCountInString(label) > 100 {
			return in, invalid()
		} else {
			in.Label = &label
		}
	}
	in.BindAddress = NormalizeAddress(in.BindAddress)
	if in.BindAddress == "" {
		in.BindAddress = DefaultBindAddress
	}
	if utf8.RuneCountInString(in.BindAddress) > 255 ||
		(!strings.EqualFold(in.BindAddress, "localhost") && net.ParseIP(in.BindAddress) == nil) ||
		(!IsLoopback(in.BindAddress) && !in.AllowNonLoopback) {
		return in, invalid()
	}
	in.DestHost = NormalizeAddress(in.DestHost)
	if utf8.RuneCountInString(in.DestHost) > 255 || !IsValidAddress(in.DestHost) ||
		validate.Int(in.BindPort, 1, 65535) != nil || validate.Int(in.DestPort, 1, 65535) != nil {
		return in, invalid()
	}
	for _, rule := range c.ForwardRules {
		if rule.ID != selfID && rule.BindAddress == in.BindAddress && rule.BindPort == in.BindPort {
			return in, invalid()
		}
	}
	return in, nil
}

func FindForwardRule(c *model.Content, id string) (model.ForwardRule, error) {
	if validate.UUID(id) != nil {
		return model.ForwardRule{}, invalid()
	}
	for _, rule := range c.ForwardRules {
		if rule.ID == id {
			return cloneForwardRule(rule), nil
		}
	}
	return model.ForwardRule{}, apperr.New(apperr.NotFound)
}

func ListForwardRulesForHost(c *model.Content, hostID string) []model.ForwardRule {
	out := make([]model.ForwardRule, 0)
	for _, rule := range c.ForwardRules {
		if rule.HostID == hostID {
			out = append(out, cloneForwardRule(rule))
		}
	}
	return out
}

func CreateForwardRule(c *model.Content, input ForwardRuleInput) (model.ForwardRule, error) {
	input, err := normalizeForwardInput(c, input, "")
	if err != nil {
		return model.ForwardRule{}, err
	}
	rule := model.ForwardRule{ID: NewID(), HostID: input.HostID, Label: input.Label,
		BindAddress: input.BindAddress, AllowNonLoopback: input.AllowNonLoopback, BindPort: input.BindPort,
		DestHost: input.DestHost, DestPort: input.DestPort}
	c.ForwardRules = append(c.ForwardRules, rule)
	return cloneForwardRule(rule), nil
}

func UpdateForwardRule(c *model.Content, id string, input ForwardRuleInput) (model.ForwardRule, error) {
	index := -1
	for i := range c.ForwardRules {
		if c.ForwardRules[i].ID == id {
			index = i
			break
		}
	}
	if index < 0 {
		return model.ForwardRule{}, apperr.New(apperr.NotFound)
	}
	input, err := normalizeForwardInput(c, input, id)
	if err != nil {
		return model.ForwardRule{}, err
	}
	rule := model.ForwardRule{ID: id, HostID: input.HostID, Label: input.Label,
		BindAddress: input.BindAddress, AllowNonLoopback: input.AllowNonLoopback, BindPort: input.BindPort,
		DestHost: input.DestHost, DestPort: input.DestPort}
	c.ForwardRules[index] = rule
	return cloneForwardRule(rule), nil
}

func RemoveForwardRule(c *model.Content, id string) error {
	for i := range c.ForwardRules {
		if c.ForwardRules[i].ID == id {
			c.ForwardRules = append(c.ForwardRules[:i], c.ForwardRules[i+1:]...)
			return nil
		}
	}
	return apperr.New(apperr.NotFound)
}

func cloneForwardRule(rule model.ForwardRule) model.ForwardRule {
	if rule.Label != nil {
		label := *rule.Label
		rule.Label = &label
	}
	return rule
}
