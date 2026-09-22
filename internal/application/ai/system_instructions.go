package ai

import (
	"fmt"
	"strings"

	"github.com/trezutil/trezutil/internal/domain"
)

type SystemInstructionsBuilder struct {
	specialists map[domain.AISpecialistCode]*domain.SystemInstructions
}

func NewSystemInstructionsBuilder() *SystemInstructionsBuilder {
	b := &SystemInstructionsBuilder{
		specialists: make(map[domain.AISpecialistCode]*domain.SystemInstructions),
	}

	for _, spec := range domain.DefaultSpecialists {
		b.specialists[spec.Code] = domain.DefaultSystemInstructions(spec.Code)
	}

	return b
}

func (b *SystemInstructionsBuilder) RegisterSpecialistInstructions(code domain.AISpecialistCode, instructions *domain.SystemInstructions) {
	b.specialists[code] = instructions
}

func (b *SystemInstructionsBuilder) Build(code domain.AISpecialistCode) *domain.SystemInstructions {
	if instr, ok := b.specialists[code]; ok {
		return instr
	}
	return domain.DefaultSystemInstructions(code)
}

func (b *SystemInstructionsBuilder) BuildForAction(action domain.CommerceActionType) *domain.SystemInstructions {
	specialists := domain.GetSpecialistsForAction(action)
	if len(specialists) == 0 {
		return b.Build(domain.AISpecialistCommerce)
	}

	primary := specialists[0]
	return b.Build(primary)
}

func (b *SystemInstructionsBuilder) BuildFullPrompt(code domain.AISpecialistCode, userContext string) string {
	instr := b.Build(code)
	var prompt strings.Builder

	prompt.WriteString(fmt.Sprintf("=== %s Instructions ===\n", code))
	prompt.WriteString(instr.SystemPrompt)
	prompt.WriteString("\n\n")

	if len(instr.Examples) > 0 {
		prompt.WriteString("=== Examples ===\n")
		for i, ex := range instr.Examples {
			prompt.WriteString(fmt.Sprintf("[%d] %s\n", i+1, ex))
		}
		prompt.WriteString("\n")
	}

	if len(instr.Constraints) > 0 {
		prompt.WriteString("=== Constraints ===\n")
		for i, c := range instr.Constraints {
			prompt.WriteString(fmt.Sprintf("[%d] %s\n", i+1, c))
		}
		prompt.WriteString("\n")
	}

	if userContext != "" {
		prompt.WriteString("=== Context ===\n")
		prompt.WriteString(userContext)
		prompt.WriteString("\n")
	}

	prompt.WriteString("=== Output Format ===\n")
	prompt.WriteString("Return ONLY a JSON object with: intent, confidence, parameters, explanation.\n")

	return prompt.String()
}

func SanitizeSystemPrompt(prompt string, sensitiveFields []string) string {
	if len(sensitiveFields) == 0 {
		sensitiveFields = domainDefaultSensitiveFields()
	}

	result := prompt
	for _, field := range sensitiveFields {
		result = strings.ReplaceAll(result, field, "[REDACTED]")
	}
	return result
}

func domainDefaultSensitiveFields() []string {
	return []string{
		"tenant_id",
		"user_id",
		"actor_id",
		"role",
		"permissions",
		"confirmation_state",
		"confirmed",
		"secret_ref",
		"password",
		"api_key",
		"credit_card",
		"ssn",
	}
}
