package main

import (
	"fmt"
	"io/ioutil"
	"encoding/json"
	"math"
	"time"
	"github.com/teambition/rrule-go"
	"strconv"
)

type WorthEvent struct {
	Name string
	ValueType string
	StaticAccountValues map[string]int
	StaticValues map[string]int
	MarginalDependentValues struct {
		DependencyName string
		DependencyOffset int
		AccountName string
		MarginalRates []struct{
			Cutoff int
			Rate float32
		}
	}
	AnnualBumpValues struct {
		BaseValues map[string]int
		AnnualBump float64
	}
	TaxableValue int
	RRULE string
}

type Account struct {
	Name string
	StartValue int
}

type Value struct {
	Name string
	StartValue int
}


func main(){
	fileContents, err := ioutil.ReadFile("events.json")
	if err != nil {
		fmt.Println("File Reading Error",err)
		return
	}
	events := make([]WorthEvent,0)
	json.Unmarshal(fileContents,&events)

	fileContents, err = ioutil.ReadFile("accounts.json")
	if err != nil {
		fmt.Println("File Reading Error",err)
		return
	}
	accounts := make([]Account,0)
	json.Unmarshal(fileContents,&accounts)

	fileContents, err = ioutil.ReadFile("values.json")
	if err != nil {
		fmt.Println("File Reading Error",err)
		return
	}
	values := make([]Value,0)
	json.Unmarshal(fileContents,&values)

	startDate := time.Date(2018, time.November, 18, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2100, time.November, 18, 0, 0, 0, 0, time.UTC)

	value := parseEvents(startDate,endDate,events,accounts,values)

	for accountName, worth := range value {
		fmt.Println(accountName+"[")
		for i:=0; i < len(worth); i=i+365 {
			fmt.Print(strconv.Itoa(worth[i])+",")
		}
	}
}

func parseEvents(firstDay time.Time, lastDay time.Time, events []WorthEvent, accounts []Account, values []Value) map[string] [] int {
	//calc the number of days to analyze to use in initializing data structure
	numDays := int(lastDay.Sub(firstDay).Nanoseconds()/86400000000000)

	//create other values field (used to hold info like taxable income)
	annualValues := make(map[string][]int,len(values))
	for _, value := range values {
		annualValues[value.Name] = make([]int,lastDay.Year()-firstDay.Year()+1)
		annualValues[value.Name][0] = value.StartValue
	}
	startYear := firstDay.Year()

	//create accountdelta structure
	accountDelta := make(map[string][]int,len(accounts))
	for _, account := range accounts {
		accountDelta[account.Name] = make([]int,numDays)
	}

	//First calculate all the deltas for each account
	for _, event := range events {
		rule, _ := rrule.StrToRRule(event.RRULE)

		occurances := rule.Between(firstDay,lastDay,true)

		for _, occurance := range occurances {
			dayOffset := int(occurance.Sub(firstDay).Nanoseconds()/86400000000000)
			switch event.ValueType {
			case "Static":
				for accountName, value := range event.StaticValues {
					accountDelta[accountName][dayOffset]+=value
				}
			case "AnnualBump":
				for accountName, value := range event.AnnualBumpValues.BaseValues {
					yearOffset := occurance.Year()-startYear
					value = int(float64(value)*math.Pow(event.AnnualBumpValues.AnnualBump,float64(yearOffset)))
					accountDelta[accountName][dayOffset]+=value
				}
			case "MarginalDependent":
				valueInfo := event.MarginalDependentValues
				dependentValue := annualValues[valueInfo.DependencyName][occurance.Year()-startYear+valueInfo.DependencyOffset]
				priorCutoff := 0
				for _, marginalRate := range valueInfo.MarginalRates {
					var valueInMargin int
					if dependentValue > marginalRate.Cutoff {
						valueInMargin = marginalRate.Cutoff - priorCutoff
					}else if dependentValue > priorCutoff {
						valueInMargin = dependentValue - priorCutoff
					}else{
						valueInMargin = 0
					}
					priorCutoff = marginalRate.Cutoff
					accountDelta[valueInfo.AccountName][dayOffset]+=int(marginalRate.Rate*float32(valueInMargin))
				}
			}
			annualValues["TaxableIncome"][occurance.Year()-startYear]+=event.TaxableValue
		}
	}

	//create accountworth structure
	accountWorth := make(map[string][]int,len(accounts))
	for _, account := range accounts {
		accountWorth[account.Name] = make([]int,numDays)
	}

	//then apply the deltas to get net value
	for accountIndex := range accounts {
		accountWorth[accounts[accountIndex].Name][0]=accounts[accountIndex].StartValue + accountDelta[accounts[accountIndex].Name][0]
		for i := 1; i < numDays; i++ {
			//if account doesn't have balance, put delta on the next account
			if accountWorth[accounts[accountIndex].Name][i-1]+accountDelta[accounts[accountIndex].Name][i]<0 && accountIndex<len(accounts)-1 {
				accountDelta[accounts[accountIndex+1].Name][i] += accountDelta[accounts[accountIndex].Name][i] - accountWorth[accounts[accountIndex].Name][i-1]
				accountWorth[accounts[accountIndex].Name][i]=0
			} else {
					accountWorth[accounts[accountIndex].Name][i] = accountWorth[accounts[accountIndex].Name][i-1] + accountDelta[accounts[accountIndex].Name][i]
			}
		}
	}

	fmt.Println(annualValues["TaxableIncome"])

	return accountWorth
}
