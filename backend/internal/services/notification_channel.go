package services

import (
	"errors"
	"fmt"

	"gorm.io/gorm"

	"capacitarr/internal/db"
	"capacitarr/internal/events"
)

// NotificationChannelService manages notification channel CRUD.
type NotificationChannelService struct {
	db  *gorm.DB
	bus *events.EventBus
}

// NewNotificationChannelService creates a new NotificationChannelService.
func NewNotificationChannelService(database *gorm.DB, bus *events.EventBus) *NotificationChannelService {
	return &NotificationChannelService{db: database, bus: bus}
}

// Create persists a new notification channel config.
func (s *NotificationChannelService) Create(config db.NotificationConfig) (*db.NotificationConfig, error) {
	if err := s.db.Create(&config).Error; err != nil {
		return nil, fmt.Errorf("failed to create notification channel: %w", err)
	}

	s.bus.Publish(events.NotificationChannelAddedEvent{
		ChannelID:   config.ID,
		ChannelType: config.Type,
		Name:        config.Name,
	})

	return &config, nil
}

// Update modifies an existing notification channel config.
func (s *NotificationChannelService) Update(id uint, config db.NotificationConfig) (*db.NotificationConfig, error) {
	var existing db.NotificationConfig
	if err := s.db.First(&existing, id).Error; err != nil {
		return nil, fmt.Errorf("notification channel not found: %w", err)
	}

	config.ID = id
	if err := s.db.Save(&config).Error; err != nil {
		return nil, fmt.Errorf("failed to update notification channel: %w", err)
	}

	s.bus.Publish(events.NotificationChannelUpdatedEvent{
		ChannelID:   config.ID,
		ChannelType: config.Type,
		Name:        config.Name,
	})

	return &config, nil
}

// Delete removes a notification channel config.
func (s *NotificationChannelService) Delete(id uint) error {
	var config db.NotificationConfig
	if err := s.db.First(&config, id).Error; err != nil {
		return fmt.Errorf("notification channel not found: %w", err)
	}

	if err := s.db.Delete(&config).Error; err != nil {
		return fmt.Errorf("failed to delete notification channel: %w", err)
	}

	s.bus.Publish(events.NotificationChannelRemovedEvent{
		ChannelID:   config.ID,
		ChannelType: config.Type,
		Name:        config.Name,
	})

	return nil
}

// List returns all notification channel configs ordered by id ascending.
func (s *NotificationChannelService) List() ([]db.NotificationConfig, error) {
	var configs []db.NotificationConfig
	if err := s.db.Order("id ASC").Find(&configs).Error; err != nil {
		return nil, fmt.Errorf("failed to list notification channels: %w", err)
	}
	return configs, nil
}

// GetByID returns a single notification channel config by primary key.
// Returns ErrNotFound if the record does not exist.
func (s *NotificationChannelService) GetByID(id uint) (*db.NotificationConfig, error) {
	var config db.NotificationConfig
	if err := s.db.First(&config, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get notification channel: %w", err)
	}
	return &config, nil
}

// ListEnabled returns all notification channel configs where enabled = true.
func (s *NotificationChannelService) ListEnabled() ([]db.NotificationConfig, error) {
	var configs []db.NotificationConfig
	if err := s.db.Where("enabled = ?", true).Find(&configs).Error; err != nil {
		return nil, fmt.Errorf("failed to list enabled notification channels: %w", err)
	}
	return configs, nil
}
