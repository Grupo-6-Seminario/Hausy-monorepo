// Command angus is the throwaway entrypoint that exercises the AgentCore harness.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/agentcore"
)

func main() {
	response, err := agentcore.Invoke(context.Background(), "Hola, como estas?")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(response)
}
