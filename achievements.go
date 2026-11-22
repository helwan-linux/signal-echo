package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Achievement struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Unlocked    bool   `json:"unlocked"`
}

type achievementSaveData struct {
	MaxUnlockedLevel int             `json:"max_unlocked_level"`
	TotalEscapes     int             `json:"total_escapes"`
	TotalDecoys      int             `json:"total_decoys"`
	UnlockedIDs      map[string]bool `json:"unlocked_ids"`
}

type AchievementManager struct {
	Items            []Achievement
	MaxUnlockedLevel int
	TotalEscapes     int
	TotalDecoys      int
	ActiveToast      *Achievement
	toastTimer       int
	savePath         string
}

func defaultAchievements() []Achievement {
	return []Achievement{
		{ID: "first_escape", Title: "First Escape", Description: "Clear any level."},
		{ID: "ghost_runner", Title: "Ghost Runner", Description: "Clear a level using 2 pulses or fewer."},
		{ID: "silent_step", Title: "Silent Step", Description: "Clear a level without sprinting."},
		{ID: "pure_instinct", Title: "Pure Instinct", Description: "Clear a level using 0 decoys."},
		{ID: "close_call", Title: "Close Call", Description: "Clear a level while an enemy is investigating."},
		{ID: "master_deceiver", Title: "Master Deceiver", Description: "Use 5 decoys total across play."},
		{ID: "campaign_clear", Title: "Campaign Clear", Description: "Reach beyond the final level."},
	}
}

func resolveSavePath() string {
	cfgDir, err := os.UserConfigDir()
	if err != nil || cfgDir == "" {
		return "signal_echo_save.json"
	}
	dir := filepath.Join(cfgDir, "signal-echo")
	_ = os.MkdirAll(dir, 0o755)
	return filepath.Join(dir, "save.json")
}

func NewAchievementManager() *AchievementManager {
	m := &AchievementManager{
		Items:    defaultAchievements(),
		savePath: resolveSavePath(),
	}
	m.load()
	return m
}

func (m *AchievementManager) load() {
	raw, err := os.ReadFile(m.savePath)
	if err != nil {
		return
	}
	var data achievementSaveData
	if err := json.Unmarshal(raw, &data); err != nil {
		return
	}
	m.MaxUnlockedLevel = data.MaxUnlockedLevel
	m.TotalEscapes = data.TotalEscapes
	m.TotalDecoys = data.TotalDecoys
	for i := range m.Items {
		if data.UnlockedIDs[m.Items[i].ID] {
			m.Items[i].Unlocked = true
		}
	}
}

func (m *AchievementManager) save() {
	unlocked := make(map[string]bool, len(m.Items))
	for _, item := range m.Items {
		if item.Unlocked {
			unlocked[item.ID] = true
		}
	}
	data := achievementSaveData{
		MaxUnlockedLevel: m.MaxUnlockedLevel,
		TotalEscapes:     m.TotalEscapes,
		TotalDecoys:      m.TotalDecoys,
		UnlockedIDs:      unlocked,
	}
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(m.savePath, raw, 0o644)
}

func (m *AchievementManager) Unlock(id string) bool {
	for i := range m.Items {
		if m.Items[i].ID == id {
			if m.Items[i].Unlocked {
				return false
			}
			m.Items[i].Unlocked = true
			cp := m.Items[i]
			m.ActiveToast = &cp
			m.toastTimer = 210
			m.save()
			return true
		}
	}
	return false
}

func (m *AchievementManager) UnlockedCount() int {
	count := 0
	for _, item := range m.Items {
		if item.Unlocked {
			count++
		}
	}
	return count
}

func (m *AchievementManager) UpdateToast() {
	if m.toastTimer > 0 {
		m.toastTimer--
		if m.toastTimer == 0 {
			m.ActiveToast = nil
		}
	}
}

func (m *AchievementManager) ResetAll() {
	m.Items = defaultAchievements()
	m.MaxUnlockedLevel = 0
	m.TotalEscapes = 0
	m.TotalDecoys = 0
	m.ActiveToast = nil
	m.toastTimer = 0
	m.save()
}