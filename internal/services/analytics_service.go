package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/SAP-F-2025/assessment-service/internal/models"
	"github.com/SAP-F-2025/assessment-service/internal/repositories"
	"github.com/SAP-F-2025/assessment-service/internal/utils"
)

// AnalyticsService provides analytics and reporting capabilities for assessments
type AnalyticsService interface {
	// Assessment Analytics
	GetAssessmentAnalytics(ctx context.Context, assessmentID uint, userID uint) (*AssessmentAnalytics, error)
	GetAssessmentPerformanceReport(ctx context.Context, assessmentID uint, userID uint) (*PerformanceReport, error)
	GetQuestionAnalytics(ctx context.Context, questionID uint, userID uint) (*QuestionAnalytics, error)

	// Student Analytics
	GetStudentPerformance(ctx context.Context, studentID uint, userID uint) (*StudentPerformance, error)
	GetStudentProgressReport(ctx context.Context, studentID uint, timeRange TimeRange, userID uint) (*ProgressReport, error)

	// Teacher Analytics
	GetTeacherDashboard(ctx context.Context, teacherID uint) (*TeacherDashboard, error)
	GetClassPerformance(ctx context.Context, classID uint, userID uint) (*ClassPerformance, error)

	// System Analytics
	GetSystemMetrics(ctx context.Context, timeRange TimeRange, userID uint) (*SystemMetrics, error)
	GetUsageStatistics(ctx context.Context, timeRange TimeRange, userID uint) (*UsageStatistics, error)

	// Comparative Analytics
	CompareAssessments(ctx context.Context, assessmentIDs []uint, userID uint) (*AssessmentComparison, error)
	GetTrendAnalysis(ctx context.Context, assessmentID uint, timeRange TimeRange, userID uint) (*TrendAnalysis, error)
}

type analyticsService struct {
	repo      repositories.Repository
	logger    *slog.Logger
	validator *utils.Validator
}

func NewAnalyticsService(repo repositories.Repository, logger *slog.Logger, validator *utils.Validator) AnalyticsService {
	return &analyticsService{
		repo:      repo,
		logger:    logger,
		validator: validator,
	}
}

// ===== DATA STRUCTURES =====

type TimeRange struct {
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
}

type AssessmentAnalytics struct {
	AssessmentID       uint                 `json:"assessment_id"`
	Title              string               `json:"title"`
	TotalAttempts      int                  `json:"total_attempts"`
	CompletedAttempts  int                  `json:"completed_attempts"`
	AverageScore       float64              `json:"average_score"`
	MedianScore        float64              `json:"median_score"`
	PassingRate        float64              `json:"passing_rate"`
	AverageTime        int                  `json:"average_time_minutes"`
	ScoreDistribution  map[string]int       `json:"score_distribution"`
	QuestionStats      []QuestionStatistics `json:"question_stats"`
	DifficultyAnalysis *DifficultyAnalysis  `json:"difficulty_analysis"`
	TimeAnalysis       *TimeAnalysis        `json:"time_analysis"`
	GeneratedAt        time.Time            `json:"generated_at"`
}

type QuestionStatistics struct {
	QuestionID          uint    `json:"question_id"`
	QuestionText        string  `json:"question_text"`
	QuestionType        string  `json:"question_type"`
	CorrectAnswers      int     `json:"correct_answers"`
	TotalAnswers        int     `json:"total_answers"`
	CorrectRate         float64 `json:"correct_rate"`
	AverageScore        float64 `json:"average_score"`
	Difficulty          string  `json:"difficulty"`
	DiscriminationIndex float64 `json:"discrimination_index"`
}

type DifficultyAnalysis struct {
	EasyQuestions   int     `json:"easy_questions"`
	MediumQuestions int     `json:"medium_questions"`
	HardQuestions   int     `json:"hard_questions"`
	EasyAvgScore    float64 `json:"easy_avg_score"`
	MediumAvgScore  float64 `json:"medium_avg_score"`
	HardAvgScore    float64 `json:"hard_avg_score"`
}

type TimeAnalysis struct {
	AverageCompletionTime int                 `json:"average_completion_time"`
	MedianCompletionTime  int                 `json:"median_completion_time"`
	TimeDistribution      map[string]int      `json:"time_distribution"`
	QuestionTimeStats     []QuestionTimeStats `json:"question_time_stats"`
}

type QuestionTimeStats struct {
	QuestionID  uint    `json:"question_id"`
	AverageTime int     `json:"average_time_seconds"`
	MedianTime  int     `json:"median_time_seconds"`
	TimeoutRate float64 `json:"timeout_rate"`
}

type PerformanceReport struct {
	AssessmentID        uint                     `json:"assessment_id"`
	Title               string                   `json:"title"`
	StudentPerformances []StudentPerformanceItem `json:"student_performances"`
	ClassStatistics     *ClassStatistics         `json:"class_statistics"`
	Recommendations     []string                 `json:"recommendations"`
	GeneratedAt         time.Time                `json:"generated_at"`
}

type StudentPerformanceItem struct {
	StudentID    uint       `json:"student_id"`
	StudentName  string     `json:"student_name"`
	AttemptCount int        `json:"attempt_count"`
	BestScore    float64    `json:"best_score"`
	LatestScore  float64    `json:"latest_score"`
	Improvement  float64    `json:"improvement"`
	Status       string     `json:"status"`
	CompletedAt  *time.Time `json:"completed_at"`
}

type ClassStatistics struct {
	TotalStudents     int     `json:"total_students"`
	CompletedStudents int     `json:"completed_students"`
	CompletionRate    float64 `json:"completion_rate"`
	ClassAverage      float64 `json:"class_average"`
	StandardDeviation float64 `json:"standard_deviation"`
	HighestScore      float64 `json:"highest_score"`
	LowestScore       float64 `json:"lowest_score"`
}

type QuestionAnalytics struct {
	QuestionID              uint               `json:"question_id"`
	QuestionText            string             `json:"question_text"`
	QuestionType            string             `json:"question_type"`
	UsageCount              int                `json:"usage_count"`
	TotalAnswers            int                `json:"total_answers"`
	CorrectAnswers          int                `json:"correct_answers"`
	CorrectRate             float64            `json:"correct_rate"`
	AverageScore            float64            `json:"average_score"`
	OptionAnalysis          []OptionAnalysis   `json:"option_analysis,omitempty"`
	CommonMistakes          []string           `json:"common_mistakes"`
	PerformanceByDifficulty map[string]float64 `json:"performance_by_difficulty"`
	GeneratedAt             time.Time          `json:"generated_at"`
}

type OptionAnalysis struct {
	OptionID    string  `json:"option_id"`
	OptionText  string  `json:"option_text"`
	SelectCount int     `json:"select_count"`
	SelectRate  float64 `json:"select_rate"`
	IsCorrect   bool    `json:"is_correct"`
}

type StudentPerformance struct {
	StudentID            uint                `json:"student_id"`
	StudentName          string              `json:"student_name"`
	TotalAssessments     int                 `json:"total_assessments"`
	CompletedAssessments int                 `json:"completed_assessments"`
	AverageScore         float64             `json:"average_score"`
	BestScore            float64             `json:"best_score"`
	WorstScore           float64             `json:"worst_score"`
	ImprovementTrend     string              `json:"improvement_trend"`
	StrengthAreas        []string            `json:"strength_areas"`
	WeaknessAreas        []string            `json:"weakness_areas"`
	RecentPerformances   []RecentPerformance `json:"recent_performances"`
	SkillAnalysis        map[string]float64  `json:"skill_analysis"`
	GeneratedAt          time.Time           `json:"generated_at"`
}

type RecentPerformance struct {
	AssessmentID    uint      `json:"assessment_id"`
	AssessmentTitle string    `json:"assessment_title"`
	Score           float64   `json:"score"`
	Percentage      float64   `json:"percentage"`
	CompletedAt     time.Time `json:"completed_at"`
	Rank            int       `json:"rank"`
}

type ProgressReport struct {
	StudentID       uint              `json:"student_id"`
	StudentName     string            `json:"student_name"`
	TimeRange       TimeRange         `json:"time_range"`
	OverallProgress *OverallProgress  `json:"overall_progress"`
	SubjectProgress []SubjectProgress `json:"subject_progress"`
	SkillProgress   []SkillProgress   `json:"skill_progress"`
	Milestones      []Milestone       `json:"milestones"`
	Recommendations []string          `json:"recommendations"`
	GeneratedAt     time.Time         `json:"generated_at"`
}

type OverallProgress struct {
	StartScore       float64 `json:"start_score"`
	EndScore         float64 `json:"end_score"`
	Improvement      float64 `json:"improvement"`
	TrendDirection   string  `json:"trend_direction"`
	ConsistencyScore float64 `json:"consistency_score"`
}

type SubjectProgress struct {
	Subject         string  `json:"subject"`
	StartScore      float64 `json:"start_score"`
	EndScore        float64 `json:"end_score"`
	Improvement     float64 `json:"improvement"`
	AssessmentCount int     `json:"assessment_count"`
}

type SkillProgress struct {
	Skill         string  `json:"skill"`
	CurrentLevel  string  `json:"current_level"`
	PreviousLevel string  `json:"previous_level"`
	Improvement   float64 `json:"improvement"`
	Mastery       float64 `json:"mastery"`
}

type Milestone struct {
	Date        time.Time `json:"date"`
	Description string    `json:"description"`
	Type        string    `json:"type"`
	Value       float64   `json:"value"`
}

type TeacherDashboard struct {
	TeacherID           uint                 `json:"teacher_id"`
	TeacherName         string               `json:"teacher_name"`
	TotalAssessments    int                  `json:"total_assessments"`
	ActiveAssessments   int                  `json:"active_assessments"`
	TotalStudents       int                  `json:"total_students"`
	PendingGrading      int                  `json:"pending_grading"`
	RecentActivity      []RecentActivity     `json:"recent_activity"`
	AssessmentSummary   []AssessmentSummary  `json:"assessment_summary"`
	StudentAlerts       []StudentAlert       `json:"student_alerts"`
	PerformanceOverview *PerformanceOverview `json:"performance_overview"`
	GeneratedAt         time.Time            `json:"generated_at"`
}

type RecentActivity struct {
	Type        string    `json:"type"`
	Description string    `json:"description"`
	Timestamp   time.Time `json:"timestamp"`
	ResourceID  uint      `json:"resource_id"`
}

type AssessmentSummary struct {
	AssessmentID   uint    `json:"assessment_id"`
	Title          string  `json:"title"`
	Status         string  `json:"status"`
	StudentCount   int     `json:"student_count"`
	CompletionRate float64 `json:"completion_rate"`
	AverageScore   float64 `json:"average_score"`
	NeedsAttention bool    `json:"needs_attention"`
}

type StudentAlert struct {
	StudentID   uint   `json:"student_id"`
	StudentName string `json:"student_name"`
	AlertType   string `json:"alert_type"`
	Message     string `json:"message"`
	Severity    string `json:"severity"`
}

type PerformanceOverview struct {
	ClassAverage       float64            `json:"class_average"`
	ImprovementRate    float64            `json:"improvement_rate"`
	CompletionRate     float64            `json:"completion_rate"`
	SubjectPerformance map[string]float64 `json:"subject_performance"`
	TrendAnalysis      string             `json:"trend_analysis"`
}

type ClassPerformance struct {
	ClassID          uint                     `json:"class_id"`
	ClassName        string                   `json:"class_name"`
	StudentCount     int                      `json:"student_count"`
	ClassStatistics  *ClassStatistics         `json:"class_statistics"`
	TopPerformers    []StudentPerformanceItem `json:"top_performers"`
	BottomPerformers []StudentPerformanceItem `json:"bottom_performers"`
	SubjectBreakdown map[string]float64       `json:"subject_breakdown"`
	GeneratedAt      time.Time                `json:"generated_at"`
}

type SystemMetrics struct {
	TimeRange          TimeRange          `json:"time_range"`
	TotalUsers         int                `json:"total_users"`
	ActiveUsers        int                `json:"active_users"`
	TotalAssessments   int                `json:"total_assessments"`
	TotalAttempts      int                `json:"total_attempts"`
	SystemLoad         *SystemLoad        `json:"system_load"`
	PerformanceMetrics *SystemPerformance `json:"performance_metrics"`
	ErrorRates         map[string]float64 `json:"error_rates"`
	UserEngagement     *UserEngagement    `json:"user_engagement"`
	GeneratedAt        time.Time          `json:"generated_at"`
}

type SystemLoad struct {
	CPUUsage    float64 `json:"cpu_usage"`
	MemoryUsage float64 `json:"memory_usage"`
	DiskUsage   float64 `json:"disk_usage"`
	NetworkIO   float64 `json:"network_io"`
}

type SystemPerformance struct {
	AverageResponseTime float64            `json:"average_response_time"`
	ThroughputRPS       float64            `json:"throughput_rps"`
	ErrorRate           float64            `json:"error_rate"`
	EndpointMetrics     map[string]float64 `json:"endpoint_metrics"`
}

type UserEngagement struct {
	DailyActiveUsers   int            `json:"daily_active_users"`
	WeeklyActiveUsers  int            `json:"weekly_active_users"`
	MonthlyActiveUsers int            `json:"monthly_active_users"`
	SessionDuration    float64        `json:"average_session_duration"`
	FeatureUsage       map[string]int `json:"feature_usage"`
}

type UsageStatistics struct {
	TimeRange              TimeRange       `json:"time_range"`
	AssessmentCreations    int             `json:"assessment_creations"`
	QuestionCreations      int             `json:"question_creations"`
	AttemptSubmissions     int             `json:"attempt_submissions"`
	UserRegistrations      int             `json:"user_registrations"`
	PopularFeatures        []FeatureUsage  `json:"popular_features"`
	PeakUsageTimes         []PeakUsageTime `json:"peak_usage_times"`
	GeographicDistribution map[string]int  `json:"geographic_distribution"`
	DeviceDistribution     map[string]int  `json:"device_distribution"`
	GeneratedAt            time.Time       `json:"generated_at"`
}

type FeatureUsage struct {
	Feature    string `json:"feature"`
	UsageCount int    `json:"usage_count"`
	UserCount  int    `json:"user_count"`
}

type PeakUsageTime struct {
	Hour       int `json:"hour"`
	UsageCount int `json:"usage_count"`
}

type AssessmentComparison struct {
	AssessmentIDs       []uint                     `json:"assessment_ids"`
	ComparisonMetrics   []AssessmentComparisonItem `json:"comparison_metrics"`
	RelativePerformance map[string]float64         `json:"relative_performance"`
	Insights            []string                   `json:"insights"`
	GeneratedAt         time.Time                  `json:"generated_at"`
}

type AssessmentComparisonItem struct {
	AssessmentID   uint    `json:"assessment_id"`
	Title          string  `json:"title"`
	AverageScore   float64 `json:"average_score"`
	PassingRate    float64 `json:"passing_rate"`
	CompletionRate float64 `json:"completion_rate"`
	Difficulty     string  `json:"difficulty"`
	StudentCount   int     `json:"student_count"`
}

type TrendAnalysis struct {
	AssessmentID    uint                 `json:"assessment_id"`
	TimeRange       TimeRange            `json:"time_range"`
	ScoreTrend      []TrendDataPoint     `json:"score_trend"`
	CompletionTrend []TrendDataPoint     `json:"completion_trend"`
	PassingTrend    []TrendDataPoint     `json:"passing_trend"`
	Seasonality     *SeasonalityAnalysis `json:"seasonality"`
	Predictions     []PredictionPoint    `json:"predictions"`
	GeneratedAt     time.Time            `json:"generated_at"`
}

type TrendDataPoint struct {
	Date  time.Time `json:"date"`
	Value float64   `json:"value"`
}

type SeasonalityAnalysis struct {
	Pattern     string   `json:"pattern"`
	Strength    float64  `json:"strength"`
	PeakPeriods []string `json:"peak_periods"`
	LowPeriods  []string `json:"low_periods"`
}

type PredictionPoint struct {
	Date       time.Time `json:"date"`
	Predicted  float64   `json:"predicted"`
	Confidence float64   `json:"confidence"`
}

// ===== ASSESSMENT ANALYTICS =====

func (s *analyticsService) GetAssessmentAnalytics(ctx context.Context, assessmentID uint, userID uint) (*AssessmentAnalytics, error) {
	s.logger.Info("Generating assessment analytics", "assessment_id", assessmentID, "user_id", userID)

	// Check permission
	assessmentService := NewAssessmentService(s.repo, s.logger, s.validator)
	canAccess, err := assessmentService.CanAccess(ctx, assessmentID, userID)
	if err != nil {
		return nil, err
	}
	if !canAccess {
		return nil, NewPermissionError(userID, assessmentID, "assessment", "view_analytics", "not owner or insufficient permissions")
	}

	// Get assessment details
	assessment, err := s.repo.Assessment().GetByIDWithDetails(ctx, assessmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get assessment: %w", err)
	}

	// Get analytics data from repository
	analyticsData, err := s.repo.Analytics().GetAssessmentAnalytics(ctx, assessmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get analytics data: %w", err)
	}

	// Build response
	analytics := &AssessmentAnalytics{
		AssessmentID:      assessmentID,
		Title:             assessment.Title,
		TotalAttempts:     analyticsData.TotalAttempts,
		CompletedAttempts: analyticsData.CompletedAttempts,
		AverageScore:      analyticsData.AverageScore,
		MedianScore:       analyticsData.MedianScore,
		PassingRate:       analyticsData.PassingRate,
		AverageTime:       analyticsData.AverageTimeMinutes,
		ScoreDistribution: analyticsData.ScoreDistribution,
		GeneratedAt:       time.Now(),
	}

	// Get question statistics
	questionStats, err := s.getQuestionStatistics(ctx, assessmentID)
	if err != nil {
		s.logger.Error("Failed to get question statistics", "error", err)
	} else {
		analytics.QuestionStats = questionStats
	}

	// Generate difficulty analysis
	analytics.DifficultyAnalysis = s.generateDifficultyAnalysis(questionStats)

	// Generate time analysis
	timeAnalysis, err := s.generateTimeAnalysis(ctx, assessmentID)
	if err != nil {
		s.logger.Error("Failed to generate time analysis", "error", err)
	} else {
		analytics.TimeAnalysis = timeAnalysis
	}

	return analytics, nil
}

func (s *analyticsService) GetAssessmentPerformanceReport(ctx context.Context, assessmentID uint, userID uint) (*PerformanceReport, error) {
	// Check permission
	assessmentService := NewAssessmentService(s.repo, s.logger, s.validator)
	canAccess, err := assessmentService.CanAccess(ctx, assessmentID, userID)
	if err != nil {
		return nil, err
	}
	if !canAccess {
		return nil, NewPermissionError(userID, assessmentID, "assessment", "view_performance", "not owner or insufficient permissions")
	}

	// Get assessment
	assessment, err := s.repo.Assessment().GetByIDWithDetails(ctx, assessmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get assessment: %w", err)
	}

	// Get student performances
	performances, err := s.getStudentPerformances(ctx, assessmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get student performances: %w", err)
	}

	// Calculate class statistics
	classStats := s.calculateClassStatistics(performances)

	// Generate recommendations
	recommendations := s.generateRecommendations(performances, classStats)

	return &PerformanceReport{
		AssessmentID:        assessmentID,
		Title:               assessment.Title,
		StudentPerformances: performances,
		ClassStatistics:     classStats,
		Recommendations:     recommendations,
		GeneratedAt:         time.Now(),
	}, nil
}

func (s *analyticsService) GetQuestionAnalytics(ctx context.Context, questionID uint, userID uint) (*QuestionAnalytics, error) {
	// Check permission
	questionService := NewQuestionService(s.repo, s.logger, s.validator)
	canAccess, err := questionService.CanAccess(ctx, questionID, userID)
	if err != nil {
		return nil, err
	}
	if !canAccess {
		return nil, NewPermissionError(userID, questionID, "question", "view_analytics", "not owner or insufficient permissions")
	}

	// Get question details
	question, err := s.repo.Question().GetByIDWithDetails(ctx, questionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get question: %w", err)
	}

	// Get analytics data
	analyticsData, err := s.repo.Analytics().GetQuestionAnalytics(ctx, questionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get question analytics: %w", err)
	}

	analytics := &QuestionAnalytics{
		QuestionID:     questionID,
		QuestionText:   question.Text,
		QuestionType:   string(question.Type),
		UsageCount:     analyticsData.UsageCount,
		TotalAnswers:   analyticsData.TotalAnswers,
		CorrectAnswers: analyticsData.CorrectAnswers,
		CorrectRate:    analyticsData.CorrectRate,
		AverageScore:   analyticsData.AverageScore,
		GeneratedAt:    time.Now(),
	}

	// Add option analysis for multiple choice questions
	if question.Type == models.QuestionTypeMultipleChoice {
		optionAnalysis, err := s.getOptionAnalysis(ctx, questionID)
		if err != nil {
			s.logger.Error("Failed to get option analysis", "error", err)
		} else {
			analytics.OptionAnalysis = optionAnalysis
		}
	}

	return analytics, nil
}

// ===== STUDENT ANALYTICS =====

func (s *analyticsService) GetStudentPerformance(ctx context.Context, studentID uint, userID uint) (*StudentPerformance, error) {
	// Check permission (student can view own performance, teachers can view their students)
	if err := s.checkStudentAnalyticsPermission(ctx, studentID, userID); err != nil {
		return nil, err
	}

	// Get student details
	student, err := s.repo.User().GetByID(ctx, studentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get student: %w", err)
	}

	// Get performance data
	performanceData, err := s.repo.Analytics().GetStudentPerformance(ctx, studentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get student performance: %w", err)
	}

	// Get recent performances
	recentPerformances, err := s.getRecentPerformances(ctx, studentID, 10)
	if err != nil {
		s.logger.Error("Failed to get recent performances", "error", err)
	}

	return &StudentPerformance{
		StudentID:            studentID,
		StudentName:          student.Name,
		TotalAssessments:     performanceData.TotalAssessments,
		CompletedAssessments: performanceData.CompletedAssessments,
		AverageScore:         performanceData.AverageScore,
		BestScore:            performanceData.BestScore,
		WorstScore:           performanceData.WorstScore,
		ImprovementTrend:     performanceData.ImprovementTrend,
		StrengthAreas:        performanceData.StrengthAreas,
		WeaknessAreas:        performanceData.WeaknessAreas,
		RecentPerformances:   recentPerformances,
		SkillAnalysis:        performanceData.SkillAnalysis,
		GeneratedAt:          time.Now(),
	}, nil
}

func (s *analyticsService) GetStudentProgressReport(ctx context.Context, studentID uint, timeRange TimeRange, userID uint) (*ProgressReport, error) {
	// Check permission
	if err := s.checkStudentAnalyticsPermission(ctx, studentID, userID); err != nil {
		return nil, err
	}

	// Get student details
	student, err := s.repo.User().GetByID(ctx, studentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get student: %w", err)
	}

	// Get progress data
	progressData, err := s.repo.Analytics().GetStudentProgress(ctx, studentID, timeRange.StartDate, timeRange.EndDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get student progress: %w", err)
	}

	return &ProgressReport{
		StudentID:       studentID,
		StudentName:     student.Name,
		TimeRange:       timeRange,
		OverallProgress: progressData.OverallProgress,
		SubjectProgress: progressData.SubjectProgress,
		SkillProgress:   progressData.SkillProgress,
		Milestones:      progressData.Milestones,
		Recommendations: progressData.Recommendations,
		GeneratedAt:     time.Now(),
	}, nil
}

// ===== TEACHER ANALYTICS =====

func (s *analyticsService) GetTeacherDashboard(ctx context.Context, teacherID uint) (*TeacherDashboard, error) {
	// Get teacher details
	teacher, err := s.repo.User().GetByID(ctx, teacherID)
	if err != nil {
		return nil, fmt.Errorf("failed to get teacher: %w", err)
	}

	// Verify teacher role
	if teacher.Role != models.RoleTeacher && teacher.Role != models.RoleAdmin {
		return nil, NewPermissionError(teacherID, 0, "dashboard", "view", "not a teacher")
	}

	// Get dashboard data
	dashboardData, err := s.repo.Analytics().GetTeacherDashboard(ctx, teacherID)
	if err != nil {
		return nil, fmt.Errorf("failed to get teacher dashboard data: %w", err)
	}

	return &TeacherDashboard{
		TeacherID:           teacherID,
		TeacherName:         teacher.Name,
		TotalAssessments:    dashboardData.TotalAssessments,
		ActiveAssessments:   dashboardData.ActiveAssessments,
		TotalStudents:       dashboardData.TotalStudents,
		PendingGrading:      dashboardData.PendingGrading,
		RecentActivity:      dashboardData.RecentActivity,
		AssessmentSummary:   dashboardData.AssessmentSummary,
		StudentAlerts:       dashboardData.StudentAlerts,
		PerformanceOverview: dashboardData.PerformanceOverview,
		GeneratedAt:         time.Now(),
	}, nil
}

func (s *analyticsService) GetClassPerformance(ctx context.Context, classID uint, userID uint) (*ClassPerformance, error) {
	// TODO: Implement class performance analytics
	// This would require a class/course system to be implemented first
	return nil, fmt.Errorf("class performance analytics not yet implemented")
}

// ===== SYSTEM ANALYTICS =====

func (s *analyticsService) GetSystemMetrics(ctx context.Context, timeRange TimeRange, userID uint) (*SystemMetrics, error) {
	// Check admin permission
	user, err := s.repo.User().GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if user.Role != models.RoleAdmin {
		return nil, NewPermissionError(userID, 0, "system", "view_metrics", "admin role required")
	}

	// Get system metrics
	metricsData, err := s.repo.Analytics().GetSystemMetrics(ctx, timeRange.StartDate, timeRange.EndDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get system metrics: %w", err)
	}

	return &SystemMetrics{
		TimeRange:          timeRange,
		TotalUsers:         metricsData.TotalUsers,
		ActiveUsers:        metricsData.ActiveUsers,
		TotalAssessments:   metricsData.TotalAssessments,
		TotalAttempts:      metricsData.TotalAttempts,
		SystemLoad:         metricsData.SystemLoad,
		PerformanceMetrics: metricsData.PerformanceMetrics,
		ErrorRates:         metricsData.ErrorRates,
		UserEngagement:     metricsData.UserEngagement,
		GeneratedAt:        time.Now(),
	}, nil
}

func (s *analyticsService) GetUsageStatistics(ctx context.Context, timeRange TimeRange, userID uint) (*UsageStatistics, error) {
	// Check admin permission
	user, err := s.repo.User().GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if user.Role != models.RoleAdmin {
		return nil, NewPermissionError(userID, 0, "system", "view_usage", "admin role required")
	}

	// Get usage statistics
	usageData, err := s.repo.Analytics().GetUsageStatistics(ctx, timeRange.StartDate, timeRange.EndDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get usage statistics: %w", err)
	}

	return &UsageStatistics{
		TimeRange:              timeRange,
		AssessmentCreations:    usageData.AssessmentCreations,
		QuestionCreations:      usageData.QuestionCreations,
		AttemptSubmissions:     usageData.AttemptSubmissions,
		UserRegistrations:      usageData.UserRegistrations,
		PopularFeatures:        usageData.PopularFeatures,
		PeakUsageTimes:         usageData.PeakUsageTimes,
		GeographicDistribution: usageData.GeographicDistribution,
		DeviceDistribution:     usageData.DeviceDistribution,
		GeneratedAt:            time.Now(),
	}, nil
}

// ===== COMPARATIVE ANALYTICS =====

func (s *analyticsService) CompareAssessments(ctx context.Context, assessmentIDs []uint, userID uint) (*AssessmentComparison, error) {
	if len(assessmentIDs) < 2 {
		return nil, NewValidationError("assessment_ids", "at least 2 assessments required for comparison", len(assessmentIDs))
	}

	// Check permission for all assessments
	assessmentService := NewAssessmentService(s.repo, s.logger, s.validator)
	for _, assessmentID := range assessmentIDs {
		canAccess, err := assessmentService.CanAccess(ctx, assessmentID, userID)
		if err != nil {
			return nil, err
		}
		if !canAccess {
			return nil, NewPermissionError(userID, assessmentID, "assessment", "compare", "access denied to one or more assessments")
		}
	}

	// Get comparison data
	comparisonData, err := s.repo.Analytics().CompareAssessments(ctx, assessmentIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get comparison data: %w", err)
	}

	// Generate insights
	insights := s.generateComparisonInsights(comparisonData.ComparisonMetrics)

	return &AssessmentComparison{
		AssessmentIDs:       assessmentIDs,
		ComparisonMetrics:   comparisonData.ComparisonMetrics,
		RelativePerformance: comparisonData.RelativePerformance,
		Insights:            insights,
		GeneratedAt:         time.Now(),
	}, nil
}

func (s *analyticsService) GetTrendAnalysis(ctx context.Context, assessmentID uint, timeRange TimeRange, userID uint) (*TrendAnalysis, error) {
	// Check permission
	assessmentService := NewAssessmentService(s.repo, s.logger, s.validator)
	canAccess, err := assessmentService.CanAccess(ctx, assessmentID, userID)
	if err != nil {
		return nil, err
	}
	if !canAccess {
		return nil, NewPermissionError(userID, assessmentID, "assessment", "view_trends", "not owner or insufficient permissions")
	}

	// Get trend data
	trendData, err := s.repo.Analytics().GetTrendAnalysis(ctx, assessmentID, timeRange.StartDate, timeRange.EndDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get trend analysis: %w", err)
	}

	return &TrendAnalysis{
		AssessmentID:    assessmentID,
		TimeRange:       timeRange,
		ScoreTrend:      trendData.ScoreTrend,
		CompletionTrend: trendData.CompletionTrend,
		PassingTrend:    trendData.PassingTrend,
		Seasonality:     trendData.Seasonality,
		Predictions:     trendData.Predictions,
		GeneratedAt:     time.Now(),
	}, nil
}

// ===== HELPER FUNCTIONS =====

func (s *analyticsService) checkStudentAnalyticsPermission(ctx context.Context, studentID uint, userID uint) error {
	// Students can view their own analytics
	if studentID == userID {
		return nil
	}

	// Teachers and admins can view student analytics
	user, err := s.repo.User().GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	if user.Role == models.RoleTeacher || user.Role == models.RoleAdmin {
		return nil
	}

	return NewPermissionError(userID, studentID, "student", "view_analytics", "insufficient permissions")
}

func (s *analyticsService) getQuestionStatistics(ctx context.Context, assessmentID uint) ([]QuestionStatistics, error) {
	// TODO: Implement question statistics retrieval
	return []QuestionStatistics{}, nil
}

func (s *analyticsService) generateDifficultyAnalysis(questionStats []QuestionStatistics) *DifficultyAnalysis {
	// TODO: Implement difficulty analysis generation
	return &DifficultyAnalysis{}
}

func (s *analyticsService) generateTimeAnalysis(ctx context.Context, assessmentID uint) (*TimeAnalysis, error) {
	// TODO: Implement time analysis generation
	return &TimeAnalysis{}, nil
}

func (s *analyticsService) getStudentPerformances(ctx context.Context, assessmentID uint) ([]StudentPerformanceItem, error) {
	// TODO: Implement student performance retrieval
	return []StudentPerformanceItem{}, nil
}

func (s *analyticsService) calculateClassStatistics(performances []StudentPerformanceItem) *ClassStatistics {
	// TODO: Implement class statistics calculation
	return &ClassStatistics{}
}

func (s *analyticsService) generateRecommendations(performances []StudentPerformanceItem, classStats *ClassStatistics) []string {
	// TODO: Implement recommendation generation
	return []string{}
}

func (s *analyticsService) getOptionAnalysis(ctx context.Context, questionID uint) ([]OptionAnalysis, error) {
	// TODO: Implement option analysis for multiple choice questions
	return []OptionAnalysis{}, nil
}

func (s *analyticsService) getRecentPerformances(ctx context.Context, studentID uint, limit int) ([]RecentPerformance, error) {
	// TODO: Implement recent performance retrieval
	return []RecentPerformance{}, nil
}

func (s *analyticsService) generateComparisonInsights(metrics []AssessmentComparisonItem) []string {
	// TODO: Implement comparison insights generation
	return []string{}
}
