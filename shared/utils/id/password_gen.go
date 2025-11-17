package id

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
)

var (
	// Word lists for generating memorable passwords
	adjectives = []string{
		"quick", "bright", "calm", "bold", "clever", "swift", "wise", "brave",
		"gentle", "happy", "proud", "strong", "noble", "keen", "fair", "warm",
		"cool", "deep", "vivid", "grand", "royal", "stellar", "cosmic", "golden",
		"silver", "crystal", "prime", "vital", "super", "ultra", "mega", "epic",
	}

	nouns = []string{
		"tiger", "eagle", "dolphin", "phoenix", "dragon", "falcon", "panther", "wolf",
		"lion", "hawk", "bear", "fox", "owl", "shark", "lynx", "raven",
		"thunder", "storm", "ocean", "mountain", "river", "forest", "sun", "moon",
		"star", "comet", "galaxy", "nebula", "aurora", "meteor", "eclipse", "horizon",
	}

	specialChars = "!@#$%&*+=?"
	digits       = "0123456789"
	lowercase    = "abcdefghijklmnopqrstuvwxyz"
	uppercase    = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
)

// GeneratePassword creates a secure, memorable password
// Format: Adjective + Noun + Number + Special
// Example: BrightTiger42!, SwiftEagle87@, BoldDragon23#
func GeneratePassword() (string, error) {
	// Select random adjective
	adjIdx, err := rand.Int(rand.Reader, big.NewInt(int64(len(adjectives))))
	if err != nil {
		return "", fmt.Errorf("failed to generate random adjective: %w", err)
	}
	adjective := adjectives[adjIdx.Int64()]

	// Select random noun
	nounIdx, err := rand.Int(rand.Reader, big.NewInt(int64(len(nouns))))
	if err != nil {
		return "", fmt.Errorf("failed to generate random noun: %w", err)
	}
	noun := nouns[nounIdx.Int64()]

	// Generate 2-digit number (10-99)
	numVal, err := rand.Int(rand.Reader, big.NewInt(90))
	if err != nil {
		return "", fmt.Errorf("failed to generate random number: %w", err)
	}
	number := fmt.Sprintf("%d", 10+numVal.Int64())

	// Select random special character
	specialIdx, err := rand.Int(rand.Reader, big.NewInt(int64(len(specialChars))))
	if err != nil {
		return "", fmt.Errorf("failed to generate random special char: %w", err)
	}
	special := string(specialChars[specialIdx.Int64()])

	// Capitalize first letter of each word
	password := capitalize(adjective) + capitalize(noun) + number + special

	return password, nil
}

// GenerateStrongPassword creates a more complex password with higher entropy
// Format: Word + Word + Number(3-4 digits) + Special(2) + Random
// Example: QuickTiger7294!@x, BrightEagle4567#$p
func GenerateStrongPassword() (string, error) {
	// Select two random words
	word1Idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(adjectives))))
	if err != nil {
		return "", err
	}
	word1 := adjectives[word1Idx.Int64()]

	word2Idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(nouns))))
	if err != nil {
		return "", err
	}
	word2 := nouns[word2Idx.Int64()]

	// Generate 3-4 digit number
	numDigits, err := rand.Int(rand.Reader, big.NewInt(2))
	if err != nil {
		return "", err
	}
	digits := 3 + int(numDigits.Int64())
	
	maxNum := big.NewInt(1)
	for i := 0; i < digits; i++ {
		maxNum.Mul(maxNum, big.NewInt(10))
	}
	numVal, err := rand.Int(rand.Reader, maxNum)
	if err != nil {
		return "", err
	}
	number := fmt.Sprintf("%0*d", digits, numVal.Int64())

	// Generate 2 special characters
	special1Idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(specialChars))))
	if err != nil {
		return "", err
	}
	special2Idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(specialChars))))
	if err != nil {
		return "", err
	}
	special := string(specialChars[special1Idx.Int64()]) + string(specialChars[special2Idx.Int64()])

	// Add one random lowercase letter at the end
	randCharIdx, err := rand.Int(rand.Reader, big.NewInt(int64(len(lowercase))))
	if err != nil {
		return "", err
	}
	randChar := string(lowercase[randCharIdx.Int64()])

	password := capitalize(word1) + capitalize(word2) + number + special + randChar

	return password, nil
}

// GenerateSimplePassword creates a simple but secure password
// Format: word + number + special
// Example: tiger42!, eagle87@, dragon23#
// Minimum length: 8 characters
func GenerateSimplePassword() (string, error) {
	// Select random noun
	nounIdx, err := rand.Int(rand.Reader, big.NewInt(int64(len(nouns))))
	if err != nil {
		return "", err
	}
	word := nouns[nounIdx.Int64()]

	// Generate 2-digit number
	numVal, err := rand.Int(rand.Reader, big.NewInt(90))
	if err != nil {
		return "", err
	}
	number := fmt.Sprintf("%d", 10+numVal.Int64())

	// Select random special character
	specialIdx, err := rand.Int(rand.Reader, big.NewInt(int64(len(specialChars))))
	if err != nil {
		return "", err
	}
	special := string(specialChars[specialIdx.Int64()])

	password := capitalize(word) + number + special

	return password, nil
}

// GeneratePassphrase creates a long, memorable passphrase
// Format: Word-Word-Word-Number
// Example: Quick-Bright-Tiger-42, Swift-Bold-Eagle-87
// Excellent for high-security scenarios, easier to remember
func GeneratePassphrase() (string, error) {
	words := make([]string, 3)
	
	for i := 0; i < 3; i++ {
		var wordList []string
		if i%2 == 0 {
			wordList = adjectives
		} else {
			wordList = nouns
		}
		
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(wordList))))
		if err != nil {
			return "", err
		}
		words[i] = capitalize(wordList[idx.Int64()])
	}

	// Generate 2-digit number
	numVal, err := rand.Int(rand.Reader, big.NewInt(90))
	if err != nil {
		return "", err
	}
	number := fmt.Sprintf("%d", 10+numVal.Int64())

	passphrase := strings.Join(words, "-") + "-" + number

	return passphrase, nil
}

// GeneratePasswordWithPolicy generates a password meeting specific requirements
type PasswordPolicy struct {
	MinLength      int
	RequireUpper   bool
	RequireLower   bool
	RequireDigit   bool
	RequireSpecial bool
	UseWords       bool // Use word-based vs random chars
}

func GeneratePasswordWithPolicy(policy PasswordPolicy) (string, error) {
	if policy.UseWords {
		// Use word-based generation for memorability
		password, err := GeneratePassword()
		if err != nil {
			return "", err
		}
		
		// Ensure minimum length
		for len(password) < policy.MinLength {
			// Add more digits
			extraDigit, err := rand.Int(rand.Reader, big.NewInt(10))
			if err != nil {
				return "", err
			}
			password += fmt.Sprintf("%d", extraDigit.Int64())
		}
		
		return password, nil
	}

	// Fallback to character-based generation
	return generateRandomPassword(policy)
}

func generateRandomPassword(policy PasswordPolicy) (string, error) {
	var charSet string
	var password strings.Builder
	
	// Build character set based on policy
	if policy.RequireUpper {
		charSet += uppercase
		// Add at least one uppercase
		idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(uppercase))))
		password.WriteByte(uppercase[idx.Int64()])
	}
	if policy.RequireLower {
		charSet += lowercase
		idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(lowercase))))
		password.WriteByte(lowercase[idx.Int64()])
	}
	if policy.RequireDigit {
		charSet += digits
		idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		password.WriteByte(digits[idx.Int64()])
	}
	if policy.RequireSpecial {
		charSet += specialChars
		idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(specialChars))))
		password.WriteByte(specialChars[idx.Int64()])
	}
	
	if charSet == "" {
		charSet = lowercase + uppercase + digits
	}
	
	// Fill remaining length
	remaining := policy.MinLength - password.Len()
	for i := 0; i < remaining; i++ {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(charSet))))
		if err != nil {
			return "", err
		}
		password.WriteByte(charSet[idx.Int64()])
	}
	
	// Shuffle the password
	passBytes := []byte(password.String())
	for i := len(passBytes) - 1; i > 0; i-- {
		jBig, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return "", err
		}
		j := int(jBig.Int64())
		passBytes[i], passBytes[j] = passBytes[j], passBytes[i]
	}
	
	return string(passBytes), nil
}

// capitalize capitalizes the first letter of a string
func capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(string(s[0])) + s[1:]
}

// Helper function to validate password strength
func ValidatePasswordStrength(password string) (strong bool, score int, feedback []string) {
	score = 0
	feedback = []string{}
	
	if len(password) >= 12 {
		score += 2
	} else if len(password) >= 8 {
		score += 1
	} else {
		feedback = append(feedback, "Password should be at least 8 characters")
	}
	
	if strings.ContainsAny(password, uppercase) {
		score += 1
	} else {
		feedback = append(feedback, "Add uppercase letters")
	}
	
	if strings.ContainsAny(password, lowercase) {
		score += 1
	} else {
		feedback = append(feedback, "Add lowercase letters")
	}
	
	if strings.ContainsAny(password, digits) {
		score += 1
	} else {
		feedback = append(feedback, "Add numbers")
	}
	
	if strings.ContainsAny(password, specialChars) {
		score += 2
	} else {
		feedback = append(feedback, "Add special characters")
	}
	
	strong = score >= 6
	return
}