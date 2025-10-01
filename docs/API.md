# Assessment Service API Documentation

## Overview

The Assessment Service provides a comprehensive REST API for managing assessments, questions, question banks, attempts, and grading. This service supports creating and managing educational assessments with various question types, automated grading, and detailed analytics.

**Base URL:** `http://localhost:8080/api/v1`

## Authentication

All API endpoints require JWT authentication. Include the JWT token in the Authorization header:

```
Authorization: Bearer <your-jwt-token>
```

## Response Format

### Success Response
```json
{
  "message": "Success message",
  "data": { ... }
}
```

### Error Response
```json
{
  "message": "Error description",
  "details": "Additional error details"
}
```

## Endpoints

### Health Check

#### GET /health
Check service health status.

**Response:**
```json
{
  "status": "healthy",
  "service": "assessment-service"
}
```

---

## Assessments

### Create Assessment

#### POST /assessments
Create a new assessment.

**Request Body:**
```json
{
  "title": "Math Quiz 1",
  "description": "Basic mathematics assessment",
  "duration": 60,
  "passing_score": 70,
  "max_attempts": 3,
  "time_warning": 300,
  "due_date": "2024-12-31T23:59:59Z",
  "settings": {
    "randomize_questions": true,
    "show_results": true,
    "allow_retake": false
  }
}
```

**Response:** `201 Created`
```json
{
  "id": 1,
  "title": "Math Quiz 1",
  "status": "Draft",
  "created_by": 123,
  "created_at": "2024-01-15T10:00:00Z"
}
```

### Get Assessment

#### GET /assessments/{id}
Retrieve assessment by ID.

**Response:** `200 OK`
```json
{
  "id": 1,
  "title": "Math Quiz 1",
  "description": "Basic mathematics assessment",
  "duration": 60,
  "status": "Active",
  "questions_count": 10,
  "total_points": 100
}
```

### Get Assessment with Details

#### GET /assessments/{id}/details
Retrieve assessment with full details including questions.

**Response:** `200 OK`
```json
{
  "id": 1,
  "title": "Math Quiz 1",
  "questions": [
    {
      "id": 1,
      "text": "What is 2 + 2?",
      "type": "multiple_choice",
      "points": 10,
      "order": 1
    }
  ],
  "settings": {
    "randomize_questions": true,
    "show_results": true
  }
}
```

### Update Assessment

#### PUT /assessments/{id}
Update assessment details.

**Request Body:**
```json
{
  "title": "Updated Math Quiz",
  "description": "Updated description",
  "duration": 90
}
```

**Response:** `200 OK`

### Delete Assessment

#### DELETE /assessments/{id}
Delete an assessment.

**Response:** `204 No Content`

### List Assessments

#### GET /assessments
List assessments with filtering.

**Query Parameters:**
- `page` (int): Page number (default: 1)
- `size` (int): Page size (default: 10)
- `status` (string): Filter by status (Draft, Active, Expired, Archived)
- `creator_id` (int): Filter by creator ID

**Response:** `200 OK`
```json
{
  "assessments": [
    {
      "id": 1,
      "title": "Math Quiz 1",
      "status": "Active",
      "questions_count": 10
    }
  ],
  "total": 1,
  "page": 1,
  "size": 10
}
```

### Search Assessments

#### GET /assessments/search
Search assessments by query.

**Query Parameters:**
- `q` (string, required): Search query
- `page` (int): Page number
- `size` (int): Page size

### Assessment Status Management

#### PUT /assessments/{id}/status
Update assessment status.

**Request Body:**
```json
{
  "status": "Active"
}
```

#### POST /assessments/{id}/publish
Publish an assessment.

#### POST /assessments/{id}/archive
Archive an assessment.

### Assessment Questions

#### POST /assessments/{id}/questions/{question_id}
Add question to assessment.

**Query Parameters:**
- `order` (int): Question order
- `points` (int): Override question points

#### DELETE /assessments/{id}/questions/{question_id}
Remove question from assessment.

#### PUT /assessments/{id}/questions/reorder
Reorder assessment questions.

**Request Body:**
```json
[
  {"question_id": 1, "order": 1},
  {"question_id": 2, "order": 2}
]
```

### Assessment Statistics

#### GET /assessments/{id}/stats
Get assessment statistics.

**Response:**
```json
{
  "total_attempts": 25,
  "completed_attempts": 20,
  "avg_score": 78.5,
  "pass_rate": 0.8,
  "avg_duration": 45
}
```

---

## Questions

### Create Question

#### POST /questions
Create a new question.

**Request Body:**
```json
{
  "type": "multiple_choice",
  "text": "What is the capital of France?",
  "points": 10,
  "difficulty": "easy",
  "category_id": 1,
  "content": {
    "options": [
      {"id": "a", "text": "London"},
      {"id": "b", "text": "Paris"},
      {"id": "c", "text": "Berlin"}
    ],
    "correct_answers": ["b"],
    "multiple_correct": false
  },
  "explanation": "Paris is the capital and largest city of France."
}
```

**Response:** `201 Created`

### Batch Create Questions

#### POST /questions/batch
Create multiple questions at once.

**Request Body:**
```json
{
  "questions": [
    {
      "type": "multiple_choice",
      "text": "Question 1",
      "content": { ... }
    },
    {
      "type": "true_false",
      "text": "Question 2",
      "content": { ... }
    }
  ]
}
```

### Get Question

#### GET /questions/{id}
Retrieve question by ID.

#### GET /questions/{id}/details
Retrieve question with full details.

### Update Question

#### PUT /questions/{id}
Update question details.

### Delete Question

#### DELETE /questions/{id}
Delete a question.

### List Questions

#### GET /questions
List questions with filtering.

**Query Parameters:**
- `page`, `size`: Pagination
- `type`: Filter by question type
- `difficulty`: Filter by difficulty level
- `category_id`: Filter by category
- `created_by`: Filter by creator

### Search Questions

#### GET /questions/search
Search questions by query.

### Random Questions

#### GET /questions/random
Get random questions for practice.

**Query Parameters:**
- `count` (int): Number of questions
- `type`: Question type filter
- `difficulty`: Difficulty filter

---

## Question Banks

### Create Question Bank

#### POST /question-banks
Create a new question bank.

**Request Body:**
```json
{
  "name": "Mathematics Bank",
  "description": "Collection of math questions",
  "is_public": false,
  "tags": ["math", "algebra"]
}
```

### Get Question Bank

#### GET /question-banks/{id}
Retrieve question bank by ID.

#### GET /question-banks/{id}/details
Retrieve question bank with questions and sharing info.

### Update Question Bank

#### PUT /question-banks/{id}
Update question bank details.

### Delete Question Bank

#### DELETE /question-banks/{id}
Delete a question bank.

### List Question Banks

#### GET /question-banks
List accessible question banks.

**Query Parameters:**
- `is_public` (bool): Filter by public banks
- `is_shared` (bool): Filter by shared banks
- `created_by` (int): Filter by creator
- `name` (string): Filter by name (partial match)

### Public Question Banks

#### GET /question-banks/public
List public question banks.

### Shared Question Banks

#### GET /question-banks/shared
List question banks shared with current user.

### Question Bank Sharing

#### POST /question-banks/{id}/share
Share question bank with users.

**Request Body:**
```json
{
  "user_ids": [123, 456],
  "permission": "read"
}
```

#### DELETE /question-banks/{id}/share/{user_id}
Remove sharing access.

#### PUT /question-banks/{id}/share/{user_id}/permissions
Update sharing permissions.

### Question Bank Management

#### POST /question-banks/{id}/questions
Add questions to bank.

**Request Body:**
```json
{
  "question_ids": [1, 2, 3]
}
```

#### DELETE /question-banks/{id}/questions
Remove questions from bank.

#### GET /question-banks/{id}/questions
Get questions in bank.

---

## Attempts

### Start Attempt

#### POST /attempts/start
Start a new assessment attempt.

**Request Body:**
```json
{
  "assessment_id": 1
}
```

**Response:**
```json
{
  "id": 1,
  "assessment_id": 1,
  "status": "in_progress",
  "started_at": "2024-01-15T10:00:00Z",
  "time_remaining": 3600
}
```

### Submit Attempt

#### POST /attempts/submit
Submit completed attempt.

**Request Body:**
```json
{
  "attempt_id": 1,
  "answers": [
    {
      "question_id": 1,
      "answer": {"selected": ["b"]}
    }
  ]
}
```

### Get Attempt

#### GET /attempts/{id}
Retrieve attempt by ID.

#### GET /attempts/{id}/details
Retrieve attempt with full details.

### Resume Attempt

#### POST /attempts/{id}/resume
Resume an in-progress attempt.

### Submit Answer

#### POST /attempts/{id}/answer
Submit answer for a specific question.

**Request Body:**
```json
{
  "question_id": 1,
  "answer": {"selected": ["b"]},
  "time_spent": 30
}
```

### Time Management

#### GET /attempts/{id}/time-remaining
Get remaining time for attempt.

#### POST /attempts/{id}/extend
Extend attempt time (admin only).

#### POST /attempts/{id}/timeout
Handle attempt timeout.

### Attempt Status

#### GET /attempts/{id}/is-active
Check if attempt is still active.

#### GET /attempts/current/{assessment_id}
Get current active attempt for assessment.

#### GET /attempts/can-start/{assessment_id}
Check if user can start new attempt.

---

## Grading

### Manual Grading

#### POST /grading/answers/{answer_id}
Grade a specific answer.

**Request Body:**
```json
{
  "score": 8,
  "max_score": 10,
  "feedback": "Good answer, minor error in calculation"
}
```

#### POST /grading/answers/batch
Grade multiple answers.

#### POST /grading/attempts/{attempt_id}
Grade entire attempt.

### Auto Grading

#### POST /grading/answers/{answer_id}/auto
Auto-grade an answer.

#### POST /grading/attempts/{attempt_id}/auto
Auto-grade entire attempt.

#### POST /grading/assessments/{assessment_id}/auto
Auto-grade all attempts for assessment.

### Grading Utilities

#### POST /grading/calculate-score
Calculate score for given answers.

#### POST /grading/generate-feedback
Generate automated feedback.

### Re-grading

#### POST /grading/questions/{question_id}/regrade
Re-grade all answers for a question.

#### POST /grading/assessments/{assessment_id}/regrade
Re-grade all attempts for assessment.

### Grading Overview

#### GET /grading/assessments/{assessment_id}/overview
Get grading overview for assessment.

---

## Error Codes

| Code | Description |
|------|-------------|
| 400 | Bad Request - Invalid input |
| 401 | Unauthorized - Authentication required |
| 403 | Forbidden - Insufficient permissions |
| 404 | Not Found - Resource not found |
| 409 | Conflict - Resource conflict |
| 422 | Unprocessable Entity - Business rule violation |
| 500 | Internal Server Error |

## Rate Limiting

- Default: 100 requests per minute per IP
- Burst: 10 requests

## Question Types

### Multiple Choice
```json
{
  "type": "multiple_choice",
  "content": {
    "options": [
      {"id": "a", "text": "Option A"},
      {"id": "b", "text": "Option B"}
    ],
    "correct_answers": ["a"],
    "multiple_correct": false
  }
}
```

### True/False
```json
{
  "type": "true_false",
  "content": {
    "correct_answer": true
  }
}
```

### Essay
```json
{
  "type": "essay",
  "content": {
    "min_words": 100,
    "max_words": 500,
    "rubric_criteria": ["Grammar", "Content", "Structure"]
  }
}
```

### Fill in the Blank
```json
{
  "type": "fill_blank",
  "content": {
    "template": "The capital of {blank1} is {blank2}",
    "blanks": {
      "blank1": {"accepted_answers": ["France"]},
      "blank2": {"accepted_answers": ["Paris"]}
    }
  }
}
```

### Short Answer
```json
{
  "type": "short_answer",
  "content": {
    "accepted_answers": ["Paris", "paris"],
    "case_sensitive": false,
    "max_length": 100
  }
}
```

## Webhooks

The service supports webhooks for real-time notifications:

- `attempt.started`
- `attempt.completed`
- `attempt.timeout`
- `assessment.published`
- `grading.completed`

Configure webhook URLs in environment variables.