package main

import (
	"log"
	"os"
	sports "temporal-sports-tracker"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func main() {
	// Create Temporal client
	c, err := client.Dial(sports.GetClientOptions())
	if err != nil {
		log.Fatalln("Unable to create Temporal client", err)
	}
	defer c.Close()

	TaskQueueName := os.Getenv("TASK_QUEUE")
	if TaskQueueName == "" {
		log.Fatalln("TASK_QUEUE environment variable is not set")
	}
	// Create worker
	w := worker.New(c, TaskQueueName, worker.Options{})

	// Register workflows
	w.RegisterWorkflow(sports.CollectGamesWorkflow)
	w.RegisterWorkflow(sports.GameWorkflow)

	// Register activities
	w.RegisterActivity(sports.GetGames)
	w.RegisterActivity(sports.StartGameWorkflow)
	w.RegisterActivity(sports.GetGameScore)
	w.RegisterActivity(sports.SendNotification)

	// Start worker
	log.Println("Starting Temporal worker for sports tracker...")
	err = w.Run(worker.InterruptCh())
	if err != nil {
		log.Fatalln("Unable to start worker", err)
	}
}
