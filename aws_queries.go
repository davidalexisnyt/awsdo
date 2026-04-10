package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
func queryRDSDatabases(profile string) ([]RDSDatabase, error) {
	commandArgs := []string{
		"rds",
		"describe-db-instances",
		"--query",
		"DBInstances[*].{ID:DBInstanceIdentifier,Endpoint:Endpoint.Address,Port:Endpoint.Port,Engine:Engine}",
		"--output=json",
	}

	if len(profile) != 0 {
		commandArgs = append(commandArgs, "--profile", profile)
	}

	command := exec.Command("aws", commandArgs...)
	outputStream, err := command.StdoutPipe()
	if err != nil {
		return nil, err
	}

	errorStream, err := command.StderrPipe()
	if err != nil {
		return nil, err
	}

	go func() {
		scanner := bufio.NewScanner(errorStream)
		scanner.Split(bufio.ScanLines)

		for scanner.Scan() {
			fmt.Println(scanner.Text())
		}
	}()

	err = command.Start()
	if err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(outputStream)
	scanner.Split(bufio.ScanLines)
	outputDoc := strings.Builder{}

	for scanner.Scan() {
		outputDoc.WriteString(strings.Trim(scanner.Text(), " "))
	}

	command.Wait()
	output := outputDoc.String()

	if len(output) == 0 {
		return []RDSDatabase{}, nil
	}

	var databases []RDSDatabase
	if err := json.Unmarshal([]byte(output), &databases); err != nil {
		return nil, fmt.Errorf("failed to parse RDS database list: %v", err)
	}

	return databases, nil
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
func queryBastionInstances(profile string, filter string) ([]EC2Instance, error) {
	if filter == "" {
		filter = "bastion"
	}

	commandArgs := []string{
		"ec2",
		"describe-instances",
		"--query",
		"Reservations[*].Instances[*].{Instance:InstanceId,AZ:Placement.AvailabilityZone,Name:Tags[?Key=='Name']|[0].Value}",
		"--filters",
		fmt.Sprintf("Name=tag:Name,Values=*%s*", filter),
		"--output=json",
	}

	if len(profile) != 0 {
		commandArgs = append(commandArgs, "--profile", profile)
	}

	command := exec.Command("aws", commandArgs...)
	outputStream, err := command.StdoutPipe()
	if err != nil {
		return nil, err
	}

	errorStream, err := command.StderrPipe()
	if err != nil {
		return nil, err
	}

	go func() {
		scanner := bufio.NewScanner(errorStream)
		scanner.Split(bufio.ScanLines)

		for scanner.Scan() {
			fmt.Println(scanner.Text())
		}
	}()

	err = command.Start()
	if err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(outputStream)
	scanner.Split(bufio.ScanLines)
	outputDoc := strings.Builder{}

	for scanner.Scan() {
		outputDoc.WriteString(strings.Trim(scanner.Text(), " "))
	}

	command.Wait()
	output := outputDoc.String()

	if len(output) == 0 {
		return []EC2Instance{}, nil
	}

	var instanceList [][]EC2Instance
	if err := json.Unmarshal([]byte(output), &instanceList); err != nil {
		return nil, fmt.Errorf("failed to parse EC2 instance list: %v", err)
	}

	var instances []EC2Instance
	for _, reservation := range instanceList {
		for _, instance := range reservation {
			if instance.Instance != "" {
				instances = append(instances, instance)
			}
		}
	}

	return instances, nil
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
func queryEC2Instances(profile string, filter string) ([]EC2Instance, error) {
	commandArgs := []string{
		"ec2",
		"describe-instances",
		"--query",
		"Reservations[*].Instances[*].{Instance:InstanceId,AZ:Placement.AvailabilityZone,Name:Tags[?Key=='Name']|[0].Value,Host:PrivateIpAddress,State:State.Name,Type:InstanceType,PublicIP:PublicIpAddress,LaunchTime:LaunchTime}",
		"--filters",
		fmt.Sprintf("Name=tag:Name,Values=*%s*", filter),
		"--output=json",
	}

	if len(profile) != 0 {
		commandArgs = append(commandArgs, "--profile", profile)
	}

	command := exec.Command("aws", commandArgs...)
	outputStream, err := command.StdoutPipe()
	if err != nil {
		return nil, err
	}

	errorStream, err := command.StderrPipe()
	if err != nil {
		return nil, err
	}

	go func() {
		scanner := bufio.NewScanner(errorStream)
		scanner.Split(bufio.ScanLines)

		for scanner.Scan() {
			fmt.Println(scanner.Text())
		}
	}()

	err = command.Start()
	if err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(outputStream)
	scanner.Split(bufio.ScanLines)
	outputDoc := strings.Builder{}

	for scanner.Scan() {
		outputDoc.WriteString(strings.Trim(scanner.Text(), " "))
	}

	command.Wait()
	output := outputDoc.String()

	if len(output) == 0 {
		return []EC2Instance{}, nil
	}

	var instanceList [][]EC2Instance
	if err := json.Unmarshal([]byte(output), &instanceList); err != nil {
		return nil, fmt.Errorf("failed to parse EC2 instance list: %v", err)
	}

	var instances []EC2Instance

	for _, reservation := range instanceList {
		for _, instance := range reservation {
			if instance.Instance != "" {
				instances = append(instances, instance)
			}
		}
	}

	return instances, nil
}
