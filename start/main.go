package main

import (
	"context"
	"fmt"
	"log"
	sports "temporal-sports-tracker"
	"time"

	"go.temporal.io/sdk/client"
)

func main() {
	c, err := client.Dial(client.Options{})
	if err != nil {
		log.Fatalln("Unable to create client", err)
	}
	defer c.Close()

	//Workflow ID is 8-digit date of now()
	//Get today's date as string
	now := time.Now()
	nowString := now.Format("20060102-150405")
	//Use that to create workflow ID
	workflowID := fmt.Sprintf("sports-%s", nowString)

	options := client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: sports.TaskQueueName,
	}

	//TODO: make variable what comes in, either a list of teams or a conference
	we, err := c.ExecuteWorkflow(context.Background(), options, sports.CollectGamesWorkflow)
	if err != nil {
		log.Fatalln("Unable to execute workflow", err)
	}
	log.Println("Started workflow", "WorkflowID", we.GetID(), "RunID", we.GetRunID())
}