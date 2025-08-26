package models

type MultipleChoiceAnswer struct {
	SelectedOptions []string `json:"selected_options"`
	TimeSpent       int      `json:"time_spent"`
}

type TrueFalseAnswer struct {
	Answer    bool `json:"answer"`
	TimeSpent int  `json:"time_spent"`
}

type EssayAnswer struct {
	Text      string `json:"text"`
	WordCount int    `json:"word_count"`
	TimeSpent int    `json:"time_spent"`
}

type FillBlankAnswer struct {
	Answers   map[string]string `json:"answers"` // blankId -> answer
	TimeSpent int               `json:"time_spent"`
}

type MatchingAnswer struct {
	Pairs     []MatchPair `json:"pairs"`
	TimeSpent int         `json:"time_spent"`
}

type OrderingAnswer struct {
	Order     []string `json:"order"` // Array of item IDs in order
	TimeSpent int      `json:"time_spent"`
}

type ShortAnswers struct {
	Text      string `json:"text"`
	TimeSpent int    `json:"time_spent"`
}
