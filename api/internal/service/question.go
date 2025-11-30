package service

import (
	"context"
	"errors"
	"log"
	"strings"
	"unicode"

	"qotd/api/internal/db"
	"qotd/api/internal/llm"
	txt "qotd/api/internal/text"
)

var (
	ErrNoQuestion       = errors.New("no question yet")
	ErrQuestionNotFound = errors.New("question not found")
	ErrGenerateFailed   = errors.New("could not generate question")
)

type GenerateResult struct {
	Question   db.Question
	Choices    []string
	Similarity float64
}

type QuestionService struct {
	repo      *db.Repository
	grader    *llm.Grader
	embedder  *llm.Embedder
	generator *llm.Generator
	logger    *log.Logger
}

func NewQuestionService(repo *db.Repository, grader *llm.Grader, embedder *llm.Embedder, generator *llm.Generator, logger *log.Logger) *QuestionService {
	return &QuestionService{repo: repo, grader: grader, embedder: embedder, generator: generator, logger: logger}
}

func (s *QuestionService) GetToday(ctx context.Context) (db.Question, error) {
	q, err := s.repo.GetLatestQuestion(ctx)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return db.Question{}, ErrNoQuestion
		}
		return db.Question{}, err
	}
	return q, nil
}

func (s *QuestionService) SubmitAnswer(ctx context.Context, questionID, answerText string) (int, string, error) {
	q, err := s.repo.GetQuestionByID(ctx, questionID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return 0, "", ErrQuestionNotFound
		}
		return 0, "", err
	}
	answerText = strings.TrimSpace(answerText)
	if len(q.Choices) > 0 {
		if matchesChoice(answerText, q.Choices) {
			const score = 10
			const feedback = "Accepted choice."
			return score, feedback, s.repo.InsertAnswer(ctx, questionID, answerText, score, nil, feedback)
		}
		resultScore := 0
		grade, err := s.grader.Grade(ctx, answerText, q.Choices)
		if err != nil {
			return 0, "", err
		}
		feedback := grade.Reason
		if grade.Match {
			matched := normalizeAnswer(grade.Choice)
			valid := false
			if matched != "" {
				for _, c := range q.Choices {
					if matched == normalizeAnswer(c) {
						valid = true
						break
					}
				}
			}
			if !valid {
				grade.Match = false
				if feedback == "" {
					feedback = "LLM match rejected: alias not in list."
				}
			}
		}
		if feedback == "" {
			feedback = "Answer not recognized as acceptable."
		}
		if grade.Match {
			resultScore = 10
			if feedback == "" {
				feedback = "Accepted choice."
			}
		}
		return resultScore, feedback, s.repo.InsertAnswer(ctx, questionID, answerText, resultScore, nil, feedback)
	}

	grade, err := s.grader.Grade(ctx, answerText, nil)
	if err != nil {
		return 0, "", err
	}
	score := 0
	feedback := grade.Reason
	if grade.Match {
		score = 10
		if feedback == "" {
			feedback = "Accepted answer."
		}
	}
	if feedback == "" {
		feedback = "Answer not recognized."
	}
	return score, feedback, s.repo.InsertAnswer(ctx, questionID, answerText, score, nil, feedback)
}

func (s *QuestionService) GenerateQuestion(ctx context.Context) (GenerateResult, error) {
	const maxTries = 5
	for i := 0; i < maxTries; i++ {
		s.logger.Printf("[generate] attempt %d/%d", i+1, maxTries)
		q, err := s.generator.GenerateQuestion(ctx)
		if err != nil {
			s.logger.Printf("[generate] llm error: %v", err)
			continue
		}
		if len(q.Text) < 20 || len(q.Text) > 400 {
			s.logger.Printf("[generate] text length out of range: %d", len(q.Text))
			continue
		}
		if len(q.Choices) == 0 {
			s.logger.Printf("[generate] no choices returned")
			continue
		}
		normalizedChoices := txt.NormalizedChoices(q.Choices)
		choiceSig := txt.ChoiceSignature(q.Choices)
		if len(normalizedChoices) > 0 {
			overlap, err := s.repo.HasChoiceOverlap(ctx, normalizedChoices)
			if err != nil {
				return GenerateResult{}, err
			}
			if overlap {
				s.logger.Printf("[generate] duplicate choices overlap detected")
				continue
			}
		}
		if choiceSig != "" {
			exists, err := s.repo.ExistsQuestionByChoiceSignature(ctx, choiceSig)
			if err != nil {
				return GenerateResult{}, err
			}
			if exists {
				s.logger.Printf("[generate] duplicate choice signature detected")
				continue
			}
		}
		norm := txt.NormalizeQuestion(q.Text)
		sha := txt.SHA256Hex(norm)
		exists, err := s.repo.ExistsQuestionBySHA(ctx, sha)
		if err != nil {
			return GenerateResult{}, err
		}
		if exists {
			s.logger.Printf("[generate] duplicate sha detected")
			continue
		}

		emb, err := s.embedder.Embed(ctx, q.Text)
		if err != nil || len(emb) == 0 {
			if err != nil {
				s.logger.Printf("[generate] embed error: %v", err)
			} else {
				s.logger.Printf("[generate] embed empty result")
			}
			continue
		}
		maxSim, err := s.repo.MaxSimilarity(ctx, emb)
		if err != nil {
			return GenerateResult{}, err
		}
		if maxSim >= 0.6 {
			s.logger.Printf("[generate] too similar: sim=%.3f", maxSim)
			continue
		}

		saved, err := s.repo.InsertQuestion(ctx, q.Title, q.Text, q.Topic, sha, emb, q.Choices, normalizedChoices, choiceSig)
		if err != nil {
			return GenerateResult{}, err
		}
		s.logger.Printf("[generate] inserted question id=%s sim=%.3f", saved.ID, maxSim)
		return GenerateResult{Question: saved, Choices: q.Choices, Similarity: maxSim}, nil
	}
	return GenerateResult{}, ErrGenerateFailed
}

func matchesChoice(input string, choices []string) bool {
	if len(choices) == 0 {
		return false
	}
	normInput := normalizeAnswer(input)
	if normInput != "" {
		for _, c := range choices {
			if normInput == normalizeAnswer(c) {
				return true
			}
		}
	}
	trimmed := strings.TrimSpace(strings.ToLower(input))
	if trimmed == "" {
		return false
	}
	labelCandidate := strings.Trim(trimmed, "(). ")
	if len(labelCandidate) == 1 {
		firstRune := []rune(labelCandidate)[0]
		if firstRune >= 'a' && firstRune <= 'z' {
			idx := int(firstRune - 'a')
			return idx >= 0 && idx < len(choices)
		}
		if firstRune >= '1' && firstRune <= '9' {
			idx := int(firstRune - '1')
			return idx >= 0 && idx < len(choices)
		}
	}
	return false
}

func normalizeAnswer(s string) string {
	if s == "" {
		return ""
	}
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.TrimPrefix(s, "the ")
	s = strings.TrimPrefix(s, "an ")
	s = strings.TrimPrefix(s, "a ")
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
