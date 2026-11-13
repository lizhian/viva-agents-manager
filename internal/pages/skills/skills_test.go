package skills

import (
	"slices"
	"testing"

	"github.com/lizhian/viva-agents-manager/internal/domain"
)

func TestSkillOrder(t *testing.T) {
	skills := []domain.Skill{
		{Name: "a-expired", Expired: true},
		{Name: "z-off"},
		{Name: "z-on", Enabled: true},
		{Name: "a-off"},
		{Name: "z-expired", Expired: true, Enabled: true},
		{Name: "a-on", Enabled: true},
	}
	slices.SortFunc(skills, compareSkills)
	want := []string{"a-on", "z-on", "a-off", "z-off", "a-expired", "z-expired"}
	for i, skill := range skills {
		if skill.Name != want[i] {
			t.Fatalf("position %d = %s, want %s", i, skill.Name, want[i])
		}
	}
}
