package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

const (
	serverAddr = "127.0.0.1:14000"
	authPass   = "my_secure_password"
	delimiter  = "<???DONE???---"
)

type Message map[string]interface{}

func sendJSONMessage(conn net.Conn, msg Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	data = append(data, []byte(delimiter)...)
	_, err = conn.Write(data)
	return err
}

func readResponse(conn net.Conn) (string, error) {
	reader := bufio.NewReader(conn)
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	var builder strings.Builder
	for {
		line, err := reader.ReadString('-')
		if err != nil {
			break
		}
		builder.WriteString(line)
		if strings.Contains(line, delimiter) {
			break
		}
	}
	full := strings.ReplaceAll(builder.String(), delimiter, "")
	return strings.TrimSpace(full), nil
}

type Cube struct {
	Name     string
	Position []float64
}

func handleCube(cube Cube, wg *sync.WaitGroup) {
	defer wg.Done()
	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		fmt.Println("[Cube] Failed to connect:", err)
		return
	}
	defer conn.Close()
	if _, err := conn.Write([]byte(authPass + delimiter)); err != nil {
		fmt.Println("[Cube] Auth write error:", err)
		return
	}
	_, err = readResponse(conn)
	if err != nil {
		fmt.Println("[Cube] Failed to read auth response:", err)
		return
	}
	spawn := Message{
		"type":      "spawn_cube",
		"cube_name": cube.Name,
		"position":  cube.Position,
		"rotation":  []float64{0, 0, 0},
		"is_base":   true,
	}
	if err := sendJSONMessage(conn, spawn); err != nil {
		fmt.Println("[Cube] Failed to spawn cube:", err)
		return
	}
	fullCubeName := cube.Name + "_BASE"
	time.Sleep(1 * time.Second)
	unfreeze := Message{
		"type":      "freeze_cube",
		"cube_name": fullCubeName,
		"freeze":    false,
	}
	if err := sendJSONMessage(conn, unfreeze); err != nil {
		fmt.Println("[Cube] Failed to unfreeze cube:", err)
		return
	}
	time.Sleep(1 * time.Second)
	freeze := Message{
		"type":      "freeze_cube",
		"cube_name": fullCubeName,
		"freeze":    true,
	}
	if err := sendJSONMessage(conn, freeze); err != nil {
		fmt.Println("[Cube] Failed to freeze cube:", err)
		return
	}
	time.Sleep(2 * time.Second)
	despawn := Message{
		"type":      "despawn_cube",
		"cube_name": fullCubeName,
	}
	if err := sendJSONMessage(conn, despawn); err != nil {
		fmt.Println("[Cube] Failed to despawn cube:", err)
		return
	}
}

func main() {
	cubeGroups := [][]Cube{
		// Ears (top layer)
		{
			{Name: "rightear", Position: []float64{0, 130, 0}},
			{Name: "leftear", Position: []float64{2, 130, 0}},
		},
		// Head (2x2 block below ears)
		{
			{Name: "head1", Position: []float64{0, 128.9, 0}},
			{Name: "head2", Position: []float64{1, 128.9, 0}},
			{Name: "head3", Position: []float64{2, 128.9, 0}},
			{Name: "head4", Position: []float64{0, 128.9, 1}},
			{Name: "head5", Position: []float64{1, 128.9, 1}},
			{Name: "head6", Position: []float64{2, 128.9, 1}},
			{Name: "head7", Position: []float64{0, 127.9, 0}},
			{Name: "head8", Position: []float64{1, 127.9, 0}},
			{Name: "head9", Position: []float64{2, 127.9, 0}},
			{Name: "head10", Position: []float64{0, 127.9, 1}},
			{Name: "head11", Position: []float64{1, 127.9, 1}},
			{Name: "head12", Position: []float64{2, 127.9, 1}},
		},
		{
			{Name: "leftmouth", Position: []float64{0.5, 128.9, -1}},
			{Name: "rightmouth", Position: []float64{1.5, 128.9, -1}},
		},
		// Neck (centered under head)
		{
			{Name: "neck", Position: []float64{1, 126.9, 0.5}},
		},
		// Body
		{
			{Name: "body1", Position: []float64{0, 125.9, 0}},
			{Name: "body2", Position: []float64{1, 125.9, 0}},
			{Name: "body3", Position: []float64{2, 125.9, 0}},
			{Name: "body4", Position: []float64{0, 125.9, 1}},
			{Name: "body5", Position: []float64{1, 125.9, 1}},
			{Name: "body6", Position: []float64{2, 125.9, 1}},
			{Name: "body7", Position: []float64{0, 124.9, 0}},
			{Name: "body8", Position: []float64{1, 124.9, 0}},
			{Name: "body9", Position: []float64{2, 124.9, 0}},
			{Name: "body10", Position: []float64{0, 124.9, 1}},
			{Name: "body11", Position: []float64{1, 124.9, 1}},
			{Name: "body12", Position: []float64{2, 124.9, 1}},

			{Name: "body13", Position: []float64{0, 125.9, 2}},
			{Name: "body14", Position: []float64{1, 125.9, 2}},
			{Name: "body15", Position: []float64{2, 125.9, 2}},
			{Name: "body16", Position: []float64{0, 125.9, 3}},
			{Name: "body17", Position: []float64{1, 125.9, 3}},
			{Name: "body18", Position: []float64{2, 125.9, 3}},
			{Name: "body19", Position: []float64{0, 124.9, 2}},
			{Name: "body20", Position: []float64{1, 124.9, 2}},
			{Name: "body21", Position: []float64{2, 124.9, 2}},
			{Name: "body22", Position: []float64{0, 124.9, 3}},
			{Name: "body23", Position: []float64{1, 124.9, 3}},
			{Name: "body24", Position: []float64{2, 124.9, 3}},
		},
	}

	var wg sync.WaitGroup
	for _, group := range cubeGroups {
		for _, cube := range group {
			wg.Add(1)
			go handleCube(cube, &wg)
		}
	}
	wg.Wait()
	time.Sleep(5 * time.Second)
}
