# Business Validator Implementation

This document outlines the business validation rules implemented according to the AS-001 documentation requirements.

## Business Rules Implemented (QT-001 to QT-009)

### QT-001: Role-based Creation Permission
- **Rule**: Only Teacher and Admin can create assessments
- **Implementation**: Handled at service layer with role checking
- **Location**: `assessment_service_helpers.go:canCreateAssessment()`

### QT-002: Teacher Ownership
- **Rule**: Teachers can only manage their own assessments
- **Implementation**: Ownership validation in permission checks
- **Location**: `assessment_service_helpers.go:CanEdit(), CanDelete()`

### QT-003: Admin Override
- **Rule**: Admins can manage all assessments
- **Implementation**: Role-based permission bypass
- **Location**: All permission check methods

### QT-004: Default Draft Status
- **Rule**: New assessments created with "Draft" status
- **Implementation**: Default value in assessment creation
- **Location**: `assessment_service.go:Create()`

### QT-005: Delete Restrictions
- **Rule**: Can only delete assessments without attempts
- **Implementation**: Business validator check
- **Location**: `business_validator.go:ValidateDeletePermission()`

### QT-006: Title Uniqueness
- **Rule**: Assessment title must be unique per creator
- **Implementation**: Repository-level uniqueness check
- **Location**: Service layer validation with repository check

### QT-007: Duration Limits
- **Rule**: Duration between 5-180 minutes
- **Implementation**: Custom validator `assessment_duration`
- **Location**: `business_validator.go:registerBusinessRules()`

### QT-008: Passing Score Range
- **Rule**: Passing score between 0-100
- **Implementation**: Custom validator `passing_score`
- **Location**: `business_validator.go:registerBusinessRules()`

### QT-009: Future Due Date
- **Rule**: Due date must be in the future
- **Implementation**: Custom validator `future_date`
- **Location**: `business_validator.go:registerBusinessRules()`

## Validation Rules Implemented (XT-001 to XT-006)

### XT-001: Title Validation
- **Rule**: Title required, 1-200 characters
- **Implementation**: Custom validator `assessment_title`
- **Validation**: `validate:"required,assessment_title"`

### XT-002: Description Validation
- **Rule**: Description max 1000 characters
- **Implementation**: Custom validator `assessment_description`
- **Validation**: `validate:"omitempty,assessment_description"`

### XT-003: Duration Validation
- **Rule**: Duration positive integer 5-180 minutes
- **Implementation**: Custom validator `assessment_duration`
- **Validation**: `validate:"required,assessment_duration"`

### XT-004: Passing Score Validation
- **Rule**: Passing score 0-100
- **Implementation**: Custom validator `passing_score`
- **Validation**: `validate:"required,passing_score"`

### XT-005: Max Attempts Validation
- **Rule**: Max attempts positive integer 1-10
- **Implementation**: Custom validator `max_attempts`
- **Validation**: `validate:"required,max_attempts"`

### XT-006: Due Date Format
- **Rule**: ISO 8601 format validation
- **Implementation**: Go time.Time type with `future_date` validator
- **Validation**: `validate:"omitempty,future_date"`

## Status Transition Validation

Based on the state diagram in docs.txt:

```
Draft → Active (publish)
Draft → Archived (archive)
Active → Expired (expire/due date)
Active → Archived (archive)
Expired → Active (reactivate)
Expired → Archived (archive)
```

**Implementation**: `ValidateStatusTransition()` method validates allowed transitions and business rules.

## Usage Examples

### Assessment Creation
```go
validator := utils.NewValidator()
req := &utils.AssessmentCreateRequest{
    Title: "Java Programming Quiz",
    Duration: 60,
    PassingScore: 70,
    MaxAttempts: 2,
}

if errors := validator.GetBusinessValidator().ValidateAssessmentCreate(req); len(errors) > 0 {
    // Handle validation errors
    for _, err := range errors {
        fmt.Printf("Field: %s, Message: %s, Rule: %s\n", err.Field, err.Message, err.Rule)
    }
}
```

### Assessment Update
```go
updateReq := &utils.AssessmentUpdateRequest{
    Title: &newTitle,
    Duration: &newDuration,
}

if errors := validator.GetBusinessValidator().ValidateAssessmentUpdate(updateReq, existingAssessment); len(errors) > 0 {
    // Handle validation errors
}
```

### Status Transition
```go
errors := validator.GetBusinessValidator().ValidateStatusTransition(
    models.StatusDraft,    // current status
    models.StatusActive,   // new status
    false,                 // has attempts
    5,                     // question count
)
```

### Delete Permission
```go
errors := validator.GetBusinessValidator().ValidateDeletePermission(
    true,                  // has attempts
    models.StatusActive,   // current status
)
```

## Integration with Services

All services now use the business validator:

1. **AssessmentService**: Uses `ValidateAssessmentCreate()` and `ValidateAssessmentUpdate()`
2. **QuestionService**: Uses `ValidateQuestionCreate()`
3. **AttemptService**: Uses `ValidateAttemptStart()`
4. **Status Management**: Uses `ValidateStatusTransition()`

## Error Response Format

```json
{
  "field": "duration",
  "message": "must be between 5 and 180 minutes",
  "value": 300,
  "rule": "QT-007"
}
```

This provides clear, actionable error messages that reference the specific business rule violated.