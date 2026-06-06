package conditional

import (
	"fmt"
	"sort"
	"strings"
)

var supportedBuildkiteEnvNames = []string{
	"BUILDKITE_BRANCH",
	"BUILDKITE_TAG",
	"BUILDKITE_MESSAGE",
	"BUILDKITE_COMMIT",
	"BUILDKITE_PIPELINE_SLUG",
	"BUILDKITE_PIPELINE_NAME",
	"BUILDKITE_PIPELINE_ID",
	"BUILDKITE_ORGANIZATION_SLUG",
	"BUILDKITE_TRIGGERED_FROM_BUILD_ID",
	"BUILDKITE_TRIGGERED_FROM_BUILD_NUMBER",
	"BUILDKITE_TRIGGERED_FROM_BUILD_PIPELINE_SLUG",
	"BUILDKITE_TRIGGERED_FROM_BUILD_JOB_ID",
	"BUILDKITE_REBUILT_FROM_BUILD_ID",
	"BUILDKITE_REBUILT_FROM_BUILD_NUMBER",
	"BUILDKITE_REPO",
	"BUILDKITE_PULL_REQUEST",
	"BUILDKITE_PULL_REQUEST_BASE_BRANCH",
	"BUILDKITE_PULL_REQUEST_REPO",
	"BUILDKITE_PULL_REQUEST_LABELS",
	"BUILDKITE_PULL_REQUEST_USING_MERGE_REFSPEC",
	"BUILDKITE_MERGE_QUEUE_BASE_BRANCH",
	"BUILDKITE_MERGE_QUEUE_BASE_COMMIT",
	"BUILDKITE_GIT_DIFF_BASE",
	"BUILDKITE_GITHUB_ACTION",
	"BUILDKITE_GITHUB_COMMENT_ID",
	"BUILDKITE_GITHUB_DEPLOYMENT_ID",
	"BUILDKITE_GITHUB_DEPLOYMENT_TASK",
	"BUILDKITE_GITHUB_DEPLOYMENT_ENVIRONMENT",
	"BUILDKITE_GITHUB_DEPLOYMENT_PAYLOAD",
	"BUILDKITE_GITHUB_EVENT",
	"BUILDKITE_GITHUB_REVIEW_ID",
	"BUILDKITE_GITHUB_CHECK_RUN_CONCLUSION",
	"BUILDKITE_GITHUB_CHECK_RUN_NAME",
	"BUILDKITE_GITHUB_DEPLOYMENT_STATUS_ENVIRONMENT",
	"BUILDKITE_GITHUB_DEPLOYMENT_STATUS_STATE",
	"BUILDKITE_GITHUB_RELEASE_DRAFT",
	"BUILDKITE_GITHUB_RELEASE_PRERELEASE",
	"BUILDKITE_GITHUB_RELEASE_TAG",
	"BUILDKITE_GITHUB_REVIEW_STATE",
}

var supportedBuildkiteEnv = stringSet(supportedBuildkiteEnvNames)

func stringSet(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		out[value] = struct{}{}
	}
	return out
}

func unsupportedBuildkiteEnv(key string) bool {
	if !strings.HasPrefix(key, "BUILDKITE_") {
		return false
	}
	_, ok := supportedBuildkiteEnv[key]
	return !ok
}

func validEnvName(key string) bool {
	if key == "" {
		return false
	}
	for _, ch := range key {
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' {
			continue
		}
		return false
	}
	return true
}

func envDollarNameMessage(key string) string {
	suggestion := strings.ReplaceAll(key, "$", "")
	if suggestion == "" {
		return "Argument to `env` should not include `$`"
	}
	return fmt.Sprintf("Argument to `env` should not include `$` - did you mean %s?", suggestion)
}

func unsupportedBuildkiteEnvMessage(key string) string {
	return fmt.Sprintf(
		"Interpolation of %q is not supported (see https://buildkite.com/docs/pipelines/environment-variables#variable-interpolation). "+
			"You can instead interpolate this variable at runtime (see https://buildkite.com/docs/pipelines/environment-variables#runtime-variable-interpolation). "+
			"If you are still having issues, please contact hello@buildkite.com.",
		key,
	)
}

func suggestBuildkiteEnv(input string) string {
	if input == "" {
		return ""
	}

	jaroThreshold := 0.77
	if len(input) > 3 {
		jaroThreshold = 0.834
	}

	type candidate struct {
		word  string
		score float64
	}
	candidates := []candidate{}
	for _, word := range supportedBuildkiteEnvNames {
		if input == word {
			continue
		}
		score := jaroWinkler(word, input)
		if score >= jaroThreshold {
			candidates = append(candidates, candidate{word: word, score: score})
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	levenshteinThreshold := (len(input) + 3) / 4
	for _, candidate := range candidates {
		if levenshtein(candidate.word, input) <= levenshteinThreshold {
			return candidate.word
		}
	}
	return ""
}

func jaroWinkler(a, b string) float64 {
	if a == b {
		return 1
	}

	left := []rune(a)
	right := []rune(b)
	if len(left) == 0 || len(right) == 0 {
		return 0
	}

	matchDistance := max(len(left), len(right))/2 - 1
	if matchDistance < 0 {
		matchDistance = 0
	}

	leftMatches := make([]bool, len(left))
	rightMatches := make([]bool, len(right))

	matches := 0
	for i, ch := range left {
		start := max(0, i-matchDistance)
		end := min(i+matchDistance+1, len(right))
		for j := start; j < end; j++ {
			if rightMatches[j] || ch != right[j] {
				continue
			}
			leftMatches[i] = true
			rightMatches[j] = true
			matches++
			break
		}
	}
	if matches == 0 {
		return 0
	}

	transpositions := 0
	j := 0
	for i, matched := range leftMatches {
		if !matched {
			continue
		}
		for !rightMatches[j] {
			j++
		}
		if left[i] != right[j] {
			transpositions++
		}
		j++
	}

	matchCount := float64(matches)
	jaro := (matchCount/float64(len(left)) +
		matchCount/float64(len(right)) +
		(matchCount-float64(transpositions)/2)/matchCount) / 3

	prefix := 0
	for prefix < min(4, min(len(left), len(right))) && left[prefix] == right[prefix] {
		prefix++
	}

	return jaro + float64(prefix)*0.1*(1-jaro)
}

func levenshtein(a, b string) int {
	left := []rune(a)
	right := []rune(b)
	if len(left) == 0 {
		return len(right)
	}
	if len(right) == 0 {
		return len(left)
	}

	previous := make([]int, len(right)+1)
	current := make([]int, len(right)+1)
	for j := range previous {
		previous[j] = j
	}

	for i, leftCh := range left {
		current[0] = i + 1
		for j, rightCh := range right {
			cost := 1
			if leftCh == rightCh {
				cost = 0
			}
			current[j+1] = min(
				previous[j+1]+1,
				current[j]+1,
				previous[j]+cost,
			)
		}
		previous, current = current, previous
	}

	return previous[len(right)]
}
