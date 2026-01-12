// internal/domain/banks.go
package domain

import (
	"fmt"
	"strings"
)

// BankInfo represents bank information
type BankInfo struct {
	Name           string
	PaybillNumber  string
	Code           string
	ShortName      string
}

// KenyanBanks is a registry of Kenyan banks with their paybill numbers
var KenyanBanks = map[string]BankInfo{
	"equity":  {
		Name:          "Equity Bank",
		PaybillNumber:  "247247",
		Code:          "68",
		ShortName:     "equity",
	},
	"kcb": {
		Name:          "KCB Bank",
		PaybillNumber: "522522",
		Code:          "01",
		ShortName:     "kcb",
	},
	"cooperative": {
		Name:          "Co-operative Bank",
		PaybillNumber: "400200",
		Code:          "11",
		ShortName:     "cooperative",
	},
	"coop": {
		Name:          "Co-operative Bank",
		PaybillNumber: "400200",
		Code:          "11",
		ShortName:     "coop",
	},
	"absa": {
		Name:          "Absa Bank Kenya",
		PaybillNumber:  "303030",
		Code:          "03",
		ShortName:     "absa",
	},
	"barclays": {
		Name:           "Absa Bank Kenya",
		PaybillNumber: "303030",
		Code:          "03",
		ShortName:     "barclays",
	},
	"dtb": {
		Name:          "Diamond Trust Bank",
		PaybillNumber: "525252",
		Code:          "63",
		ShortName:     "dtb",
	},
	"diamond": {
		Name:          "Diamond Trust Bank",
		PaybillNumber: "525252",
		Code:          "63",
		ShortName:     "diamond",
	},
	"ncba": {
		Name:          "NCBA Bank",
		PaybillNumber: "202202",
		Code:          "07",
		ShortName:     "ncba",
	},
	"stanbic": {
		Name:           "Stanbic Bank",
		PaybillNumber: "567567",
		Code:          "31",
		ShortName:     "stanbic",
	},
	"standard": {
		Name:          "Standard Chartered Bank",
		PaybillNumber: "329329",
		Code:          "02",
		ShortName:     "standard",
	},
	"scb": {
		Name:          "Standard Chartered Bank",
		PaybillNumber: "329329",
		Code:          "02",
		ShortName:     "scb",
	},
	"family": {
		Name:          "Family Bank",
		PaybillNumber: "222222",
		Code:          "70",
		ShortName:     "family",
	},
	"imebank": {
		Name:          "I&M Bank",
		PaybillNumber: "300300",
		Code:          "57",
		ShortName:     "imebank",
	},
	"guaranty": {
		Name:          "Guaranty Trust Bank",
		PaybillNumber: "222226",
		Code:          "53",
		ShortName:     "guaranty",
	},
	"gtbank": {
		Name:          "Guaranty Trust Bank",
		PaybillNumber: "222226",
		Code:          "53",
		ShortName:     "gtbank",
	},
	"sidian": {
		Name:          "Sidian Bank",
		PaybillNumber: "920920",
		Code:          "66",
		ShortName:     "sidian",
	},
	"nationalbank": {
		Name:          "National Bank of Kenya",
		PaybillNumber: "220220",
		Code:          "12",
		ShortName:     "nationalbank",
	},
	"citibank": {
		Name:           "Citibank",
		PaybillNumber: "254254",
		Code:          "16",
		ShortName:     "citibank",
	},
	"uba": {
		Name:          "UBA Kenya Bank",
		PaybillNumber:  "777777",
		Code:          "76",
		ShortName:     "uba",
	},
	"ecobank": {
		Name:           "Ecobank Kenya",
		PaybillNumber: "247240",
		Code:          "43",
		ShortName:     "ecobank",
	},
	"hfc": {
		Name:          "HF Group",
		PaybillNumber: "220222",
		Code:          "61",
		ShortName:     "hfc",
	},
	"credit": {
		Name:          "Credit Bank",
		PaybillNumber: "222223",
		Code:          "25",
		ShortName:     "credit",
	},
	"prime": {
		Name:          "Prime Bank",
		PaybillNumber: "880880",
		Code:          "10",
		ShortName:     "prime",
	},
	"victoria": {
		Name:          "Victoria Commercial Bank",
		PaybillNumber: "211211",
		Code:          "54",
		ShortName:     "victoria",
	},
	"guardian": {
		Name:          "Guardian Bank",
		PaybillNumber: "808808",
		Code:          "55",
		ShortName:     "guardian",
	},
}

// GetBankByName retrieves bank info by name (case-insensitive, fuzzy match)
func GetBankByName(bankName string) (*BankInfo, error) {
	bankName = strings.TrimSpace(strings.ToLower(bankName))
	
	// Direct match
	if bank, exists := KenyanBanks[bankName]; exists {
		return &bank, nil
	}
	
	// Fuzzy match - check if any bank name contains the search term
	for key, bank := range KenyanBanks {
		if strings.Contains(strings.ToLower(bank.Name), bankName) ||
			strings.Contains(key, bankName) {
			return &bank, nil
		}
	}
	
	return nil, fmt.Errorf("bank not found: %s", bankName)
}

// GetAllBanks returns all registered banks
func GetAllBanks() []BankInfo {
	seen := make(map[string]bool)
	var banks []BankInfo
	
	for _, bank := range KenyanBanks {
		// Avoid duplicates (same bank with different aliases)
		if !seen[bank.PaybillNumber] {
			banks = append(banks, bank)
			seen[bank.PaybillNumber] = true
		}
	}
	
	return banks
}

// ValidateBankAccount validates bank account format
func ValidateBankAccount(bankAccount string) (bankName, accountNumber string, err error) {
	parts := strings.SplitN(bankAccount, ",", 2)
	if len(parts) != 2 {
		return "", "", fmt. Errorf("invalid bank_account format, expected 'bank_name,account_number'")
	}
	
	bankName = strings.TrimSpace(parts[0])
	accountNumber = strings.TrimSpace(parts[1])
	
	if bankName == "" {
		return "", "", fmt. Errorf("bank name is empty")
	}
	
	if accountNumber == "" {
		return "", "", fmt.Errorf("account number is empty")
	}
	
	return bankName, accountNumber, nil
}