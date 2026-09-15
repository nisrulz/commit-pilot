package lib

import (
	"context"
	"os"
	"time"
)

func applyFileConfig(c *Config, fc fileConfig) {
	if fc.Provider != "" {
		c.Provider = fc.Provider
	}
	if fc.Model != "" {
		c.Model = fc.Model
	}
	if fc.BaseURL != "" {
		c.APIBase = fc.BaseURL
	}
	if fc.ContextWindow != nil {
		c.ContextWindow = *fc.ContextWindow
		c.contextSet = true
	}
	if fc.Prompt != "" {
		c.Prompt = fc.Prompt
	}
	if fc.Mode != "" {
		switch fc.Mode {
		case "single":
			c.Mode = ModeSingle
		case "auto":
			c.Mode = ModeAuto
		default:
			c.Mode = Mode(fc.Mode)
		}
	}
	if fc.DryRun != nil {
		c.DryRun = *fc.DryRun
	}
	if fc.Cleanup != nil {
		c.Cleanup = *fc.Cleanup
	}
	if fc.Retries != nil {
		c.Retries = *fc.Retries
		c.retriesSet = true
	}
	if fc.TimeoutSeconds != nil {
		c.Timeout = time.Duration(*fc.TimeoutSeconds) * time.Second
		c.timeoutSet = true
	}
	if fc.Conventional != nil {
		c.Conventional = *fc.Conventional
		c.conventionalSet = true
	}
	if fc.TicketPrefix != "" {
		c.TicketPrefix = fc.TicketPrefix
	}
	if fc.Imperative != nil {
		c.Imperative = *fc.Imperative
		c.imperativeSet = true
	}
	if fc.MaxSubjectLength != nil {
		c.MaxSubjectLength = *fc.MaxSubjectLength
	}
	if fc.BodyStyle != "" {
		c.BodyStyle = fc.BodyStyle
	}
	if fc.DefaultBranch != "" {
		c.DefaultBranch = fc.DefaultBranch
	}
	if fc.OutputFormat != "" {
		c.OutputFormat = fc.OutputFormat
	}
}

func applyRepoConfig(c *Config, rc repoConfig) {
	if rc.Conventional != nil {
		c.Conventional = *rc.Conventional
		c.conventionalSet = true
	}
	if rc.TicketPrefix != "" {
		c.TicketPrefix = rc.TicketPrefix
	}
	if rc.Imperative != nil {
		c.Imperative = *rc.Imperative
		c.imperativeSet = true
	}
	if rc.MaxSubjectLength != nil {
		c.MaxSubjectLength = *rc.MaxSubjectLength
	}
	if rc.BodyStyle != "" {
		c.BodyStyle = rc.BodyStyle
	}
	if rc.DefaultBranch != "" {
		c.DefaultBranch = rc.DefaultBranch
	}
	if rc.OutputFormat != "" {
		c.OutputFormat = rc.OutputFormat
	}
}

func applyFlags(c *Config, f RawFlags) {
	if f.DryRun {
		c.DryRun = true
	}
	if f.Cleanup {
		c.Cleanup = true
	}
	if f.Yes {
		c.Yes = true
	}
	if f.Mode == "1" {
		c.Mode = ModeSingle
	}
	if f.Staged {
		c.Scope = ScopeStaged
	} else if f.Unstaged {
		c.Scope = ScopeUnstaged
	}
	c.PlanOut = f.PlanOut
	c.Apply = f.Apply
	c.PlanLint = f.PlanLint
	c.Include = f.Include
	c.Exclude = f.Exclude
	c.IncludeSensitive = f.IncludeSensitive
	c.JSON = f.JSON
	c.Quiet = f.Quiet
}

func applyProfileDefaults(c *Config) {
	if c.Provider == "" {
		c.Provider = DefaultProviderName
	}
	if c.APIBase == "" {
		c.APIBase = KnownProviders[c.Provider]
	}
	if c.APIBase == "" {
		c.APIBase = DefaultAPIBase
	}
	if c.Model == "" {
		c.Model = ProviderDefaults[c.Provider]
	}
	if c.Model == "" {
		c.Model = DefaultModel
	}
	if !c.contextSet {
		c.ContextWindow = defaultContextWindow
	}
	if !c.retriesSet {
		c.Retries = defaultRetries
	}
	if !c.timeoutSet {
		c.Timeout = defaultTimeout
	}
	if !c.conventionalSet {
		c.Conventional = true
	}
	if !c.imperativeSet {
		c.Imperative = true
	}
	if c.MaxSubjectLength == 0 {
		c.MaxSubjectLength = MaxSubjectLength
	}
	if c.Context == nil {
		c.Context = context.Background()
	}
	if c.Input == nil {
		c.Input = os.Stdin
	}
	if c.Output == nil {
		c.Output = os.Stdout
	}
}
