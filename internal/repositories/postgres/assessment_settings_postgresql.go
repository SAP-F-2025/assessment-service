package postgres

import (
	"context"

	"github.com/SAP-F-2025/assessment-service/internal/cache"
	"github.com/SAP-F-2025/assessment-service/internal/models"
	"gorm.io/gorm"
)

type AssessmentSettingsPostgreSQL struct {
	db           *gorm.DB
	helpers      *SharedHelpers
	cacheManager *cache.CacheManager
}

func (a AssessmentSettingsPostgreSQL) Create(ctx context.Context, tx *gorm.DB, settings *models.AssessmentSettings) error {
	db := a.getDB(tx)
	return db.WithContext(ctx).Create(settings).Error
}

func (a AssessmentSettingsPostgreSQL) GetByAssessmentID(ctx context.Context, tx *gorm.DB, assessmentID uint) (*models.AssessmentSettings, error) {
	db := a.getDB(tx)
	var settings models.AssessmentSettings
	if err := db.WithContext(ctx).Where("assessment_id = ?", assessmentID).First(&settings).Error; err != nil {
		return nil, err
	}
	return &settings, nil
}

func (a AssessmentSettingsPostgreSQL) Update(ctx context.Context, tx *gorm.DB, settings *models.AssessmentSettings) error {
	db := a.getDB(tx)
	return db.WithContext(ctx).Save(settings).Error
}

func (a AssessmentSettingsPostgreSQL) Delete(ctx context.Context, tx *gorm.DB, assessmentID uint) error {
	db := a.getDB(tx)
	return db.WithContext(ctx).Where("assessment_id = ?", assessmentID).Delete(&models.AssessmentSettings{}).Error
}

func (a AssessmentSettingsPostgreSQL) CreateDefault(ctx context.Context, tx *gorm.DB, assessmentID uint) error {
	db := a.getDB(tx)
	defaultSettings := models.AssessmentSettings{
		AssessmentID: assessmentID,
		// Set other default fields as necessary
	}
	return db.WithContext(ctx).Create(&defaultSettings).Error
}

func (a AssessmentSettingsPostgreSQL) GetMultiple(ctx context.Context, tx *gorm.DB, assessmentIDs []uint) (map[uint]*models.AssessmentSettings, error) {
	db := a.getDB(tx)
	var settings []models.AssessmentSettings
	if err := db.WithContext(ctx).Where("assessment_id IN ?", assessmentIDs).Find(&settings).Error; err != nil {
		return nil, err
	}

	settingsMap := make(map[uint]*models.AssessmentSettings)
	for _, s := range settings {
		sCopy := s
		settingsMap[s.AssessmentID] = &sCopy
	}

	return settingsMap, nil
}

func (a AssessmentSettingsPostgreSQL) getDB(tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx
	}
	return a.db
}
