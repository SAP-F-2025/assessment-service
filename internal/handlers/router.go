package handlers

import (
	"github.com/SAP-F-2025/assessment-service/internal/services"
	"github.com/SAP-F-2025/assessment-service/internal/utils"
	"github.com/gin-gonic/gin"
)

type HandlerManager struct {
	assessmentHandler   *AssessmentHandler
	questionHandler     *QuestionHandler
	questionBankHandler *QuestionBankHandler
	attemptHandler      *AttemptHandler
	gradingHandler      *GradingHandler
}

func NewHandlerManager(
	serviceManager services.ServiceManager,
	validator *utils.Validator,
	logger utils.Logger,
) *HandlerManager {
	return &HandlerManager{
		assessmentHandler:   NewAssessmentHandler(serviceManager.Assessment(), validator, logger),
		questionHandler:     NewQuestionHandler(serviceManager.Question(), validator, logger),
		questionBankHandler: NewQuestionBankHandler(serviceManager.QuestionBank(), logger),
		attemptHandler:      NewAttemptHandler(serviceManager.Attempt(), validator, logger),
		gradingHandler:      NewGradingHandler(serviceManager.Grading(), validator, logger),
	}
}

// SetupRoutes sets up all API routes
func (hm *HandlerManager) SetupRoutes(router *gin.Engine) {
	// Health check endpoint
	router.GET("/health", HealthCheck)

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Assessment routes
		assessments := v1.Group("/assessments")
		{
			assessments.POST("", hm.assessmentHandler.CreateAssessment)
			assessments.GET("", hm.assessmentHandler.ListAssessments)
			assessments.GET("/search", hm.assessmentHandler.SearchAssessments)
			assessments.GET("/:id", hm.assessmentHandler.GetAssessment)
			assessments.GET("/:id/details", hm.assessmentHandler.GetAssessmentWithDetails)
			assessments.PUT("/:id", hm.assessmentHandler.UpdateAssessment)
			assessments.DELETE("/:id", hm.assessmentHandler.DeleteAssessment)
			assessments.PUT("/:id/status", hm.assessmentHandler.UpdateAssessmentStatus)
			assessments.POST("/:id/publish", hm.assessmentHandler.PublishAssessment)
			assessments.POST("/:id/archive", hm.assessmentHandler.ArchiveAssessment)
			assessments.GET("/:id/stats", hm.assessmentHandler.GetAssessmentStats)

			// Assessment question management
			assessments.POST("/:id/questions/:question_id", hm.assessmentHandler.AddQuestionToAssessment)
			assessments.DELETE("/:id/questions/:question_id", hm.assessmentHandler.RemoveQuestionFromAssessment)
			assessments.PUT("/:id/questions/reorder", hm.assessmentHandler.ReorderAssessmentQuestions)

			// Creator-specific routes
			assessments.GET("/creator/:creator_id", hm.assessmentHandler.GetAssessmentsByCreator)
			assessments.GET("/creator/:creator_id/stats", hm.assessmentHandler.GetCreatorStats)
		}

		// Question routes
		questions := v1.Group("/questions")
		{
			questions.POST("", hm.questionHandler.CreateQuestion)
			questions.POST("/batch", hm.questionHandler.CreateQuestionsBatch)
			questions.PUT("/batch", hm.questionHandler.UpdateQuestionsBatch)
			questions.GET("", hm.questionHandler.ListQuestions)
			questions.GET("/search", hm.questionHandler.SearchQuestions)
			questions.GET("/random", hm.questionHandler.GetRandomQuestions)
			questions.GET("/:id", hm.questionHandler.GetQuestion)
			questions.GET("/:id/details", hm.questionHandler.GetQuestionWithDetails)
			questions.PUT("/:id", hm.questionHandler.UpdateQuestion)
			questions.DELETE("/:id", hm.questionHandler.DeleteQuestion)
			questions.GET("/:id/stats", hm.questionHandler.GetQuestionStats)

			// Question bank management
			questions.GET("/bank/:bank_id", hm.questionHandler.GetQuestionsByBank)
			questions.POST("/:id/bank/:bank_id", hm.questionHandler.AddQuestionToBank)
			questions.DELETE("/:id/bank/:bank_id", hm.questionHandler.RemoveQuestionFromBank)

			// Creator-specific routes
			questions.GET("/creator/:creator_id", hm.questionHandler.GetQuestionsByCreator)
			questions.GET("/creator/:creator_id/usage-stats", hm.questionHandler.GetQuestionUsageStats)
		}

		// Question Bank routes
		questionBanks := v1.Group("/question-banks")
		{
			questionBanks.POST("", hm.questionBankHandler.CreateQuestionBank)
			questionBanks.GET("", hm.questionBankHandler.ListQuestionBanks)
			questionBanks.GET("/public", hm.questionBankHandler.GetPublicQuestionBanks)
			questionBanks.GET("/shared", hm.questionBankHandler.GetSharedQuestionBanks)
			questionBanks.GET("/search", hm.questionBankHandler.SearchQuestionBanks)
			questionBanks.GET("/:id", hm.questionBankHandler.GetQuestionBank)
			questionBanks.GET("/:id/details", hm.questionBankHandler.GetQuestionBankWithDetails)
			questionBanks.PUT("/:id", hm.questionBankHandler.UpdateQuestionBank)
			questionBanks.DELETE("/:id", hm.questionBankHandler.DeleteQuestionBank)
			questionBanks.GET("/:id/stats", hm.questionBankHandler.GetQuestionBankStats)

			// Sharing management
			questionBanks.POST("/:id/share", hm.questionBankHandler.ShareQuestionBank)
			questionBanks.DELETE("/:id/share/:user_id", hm.questionBankHandler.UnshareQuestionBank)
			questionBanks.PUT("/:id/share/:user_id/permissions", hm.questionBankHandler.UpdateSharePermissions)
			questionBanks.GET("/:id/shares", hm.questionBankHandler.GetQuestionBankShares)
			questionBanks.GET("/user/:user_id/shares", hm.questionBankHandler.GetUserShares)

			// Question management
			questionBanks.POST("/:id/questions", hm.questionBankHandler.AddQuestionsToBank)
			questionBanks.DELETE("/:id/questions", hm.questionBankHandler.RemoveQuestionsFromBank)
			questionBanks.GET("/:id/questions", hm.questionBankHandler.GetBankQuestions)

			// Creator-specific routes
			questionBanks.GET("/creator/:creator_id", hm.questionBankHandler.GetQuestionBanksByCreator)
		}

		// Attempt routes
		attempts := v1.Group("/attempts")
		{
			attempts.POST("/start", hm.attemptHandler.StartAttempt)
			attempts.POST("/submit", hm.attemptHandler.SubmitAttempt)
			attempts.GET("", hm.attemptHandler.ListAttempts)
			attempts.GET("/:id", hm.attemptHandler.GetAttempt)
			attempts.GET("/:id/details", hm.attemptHandler.GetAttemptWithDetails)
			attempts.POST("/:id/resume", hm.attemptHandler.ResumeAttempt)
			attempts.POST("/:id/answer", hm.attemptHandler.SubmitAnswer)
			attempts.GET("/:id/time-remaining", hm.attemptHandler.GetTimeRemaining)
			attempts.POST("/:id/extend", hm.attemptHandler.ExtendTime)
			attempts.POST("/:id/timeout", hm.attemptHandler.HandleTimeout)
			attempts.GET("/:id/is-active", hm.attemptHandler.IsAttemptActive)

			// Assessment-specific routes
			attempts.GET("/current/:assessment_id", hm.attemptHandler.GetCurrentAttempt)
			attempts.GET("/can-start/:assessment_id", hm.attemptHandler.CanStartAttempt)
			attempts.GET("/count/:assessment_id", hm.attemptHandler.GetAttemptCount)
			attempts.GET("/assessment/:assessment_id", hm.attemptHandler.GetAttemptsByAssessment)
			attempts.GET("/stats/:assessment_id", hm.attemptHandler.GetAttemptStats)

			// Student-specific routes
			attempts.GET("/student/:student_id", hm.attemptHandler.GetAttemptsByStudent)
		}

		// Grading routes
		grading := v1.Group("/grading")
		{
			// Manual grading
			grading.POST("/answers/:answer_id", hm.gradingHandler.GradeAnswer)
			grading.POST("/answers/batch", hm.gradingHandler.GradeMultipleAnswers)
			grading.POST("/attempts/:attempt_id", hm.gradingHandler.GradeAttempt)

			// Auto grading
			grading.POST("/answers/:answer_id/auto", hm.gradingHandler.AutoGradeAnswer)
			grading.POST("/attempts/:attempt_id/auto", hm.gradingHandler.AutoGradeAttempt)
			grading.POST("/assessments/:assessment_id/auto", hm.gradingHandler.AutoGradeAssessment)

			// Grading utilities
			grading.POST("/calculate-score", hm.gradingHandler.CalculateScore)
			grading.POST("/generate-feedback", hm.gradingHandler.GenerateFeedback)

			// Re-grading
			grading.POST("/questions/:question_id/regrade", hm.gradingHandler.ReGradeQuestion)
			grading.POST("/assessments/:assessment_id/regrade", hm.gradingHandler.ReGradeAssessment)

			// Grading overview
			grading.GET("/assessments/:assessment_id/overview", hm.gradingHandler.GetGradingOverview)
		}
	}

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "healthy",
			"service": "assessment-service",
		})
	})
}

// AdminMiddleware - placeholder for admin authorization middleware
func AdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: Implement admin authorization logic
		c.Next()
	}
}
