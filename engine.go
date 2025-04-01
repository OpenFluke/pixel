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

type Cube struct {
	Name     string
	Position []float64
}

var (
	globalCubeList []string
	cubeListMutex  sync.Mutex
)

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

func spawnCube(cube Cube, wg *sync.WaitGroup) {
	defer wg.Done()
	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		fmt.Println("[Spawn] Failed to connect:", err)
		return
	}
	defer conn.Close()

	if _, err := conn.Write([]byte(authPass + delimiter)); err != nil {
		fmt.Println("[Spawn] Auth write error:", err)
		return
	}
	_, err = readResponse(conn)
	if err != nil {
		fmt.Println("[Spawn] Failed to read auth response:", err)
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
		fmt.Println("[Spawn] Failed to spawn cube:", err)
		return
	}

	fullCubeName := cube.Name + "_BASE"
	cubeListMutex.Lock()
	globalCubeList = append(globalCubeList, fullCubeName)
	cubeListMutex.Unlock()
}

func unfreezeAllCubes() {
	var wg sync.WaitGroup
	for _, cube := range globalCubeList {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			conn, err := net.Dial("tcp", serverAddr)
			if err != nil {
				fmt.Println("[Unfreeze] Failed to connect:", err)
				return
			}
			defer conn.Close()

			if _, err := conn.Write([]byte(authPass + delimiter)); err != nil {
				return
			}
			_, _ = readResponse(conn)

			unfreeze := Message{
				"type":      "freeze_cube",
				"cube_name": name,
				"freeze":    false,
			}
			sendJSONMessage(conn, unfreeze)
		}(cube)
	}
	wg.Wait()
}

func despawnAllCubes() {
	var wg sync.WaitGroup
	for _, cube := range globalCubeList {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			conn, err := net.Dial("tcp", serverAddr)
			if err != nil {
				fmt.Println("[Despawn] Failed to connect:", err)
				return
			}
			defer conn.Close()

			if _, err := conn.Write([]byte(authPass + delimiter)); err != nil {
				return
			}
			_, _ = readResponse(conn)

			despawn := Message{
				"type":      "despawn_cube",
				"cube_name": name,
			}
			sendJSONMessage(conn, despawn)
		}(cube)
	}
	wg.Wait()
}

func main() {
	cubeGroups := [][]Cube{
		{
			{Name: "rightear", Position: []float64{0, 130, 0}},
			{Name: "leftear", Position: []float64{2, 130, 0}},
		},
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
		{
			{Name: "neck", Position: []float64{1, 126.9, 0.5}},
		},
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
		{
			{Name: "leftbackleg1", Position: []float64{2, 123.7, 3}},
			{Name: "leftbackknee1", Position: []float64{2, 122.5, 3}},
			{Name: "leftbackleg2", Position: []float64{2, 121.3, 3}},
		},
		{
			{Name: "leftfrontleg1", Position: []float64{2, 123.7, 0}},
			{Name: "leftfrontknee1", Position: []float64{2, 122.5, 0}},
			{Name: "leftfrontleg2", Position: []float64{2, 121.3, 0}},
		},
		{
			{Name: "rightbackleg1", Position: []float64{0, 123.7, 3}},
			{Name: "rightbackknee1", Position: []float64{0, 122.5, 3}},
			{Name: "rightbackleg2", Position: []float64{0, 121.3, 3}},
		},
		{
			{Name: "rightfrontleg1", Position: []float64{0, 123.7, 0}},
			{Name: "rightfrontknee1", Position: []float64{0, 122.5, 0}},
			{Name: "rightfrontleg2", Position: []float64{0, 121.3, 0}},
		},
		{
			{Name: "tail1", Position: []float64{1, 127.2, 3}},
			{Name: "tail2", Position: []float64{1, 128.4, 3}},
			{Name: "tail3", Position: []float64{1, 129.6, 3}},
		},
	}

	var wg sync.WaitGroup
	for _, group := range cubeGroups {
		for _, cube := range group {
			wg.Add(1)
			go spawnCube(cube, &wg)
		}
	}
	wg.Wait()

	fmt.Println("Spawned all cubes.")
	unfreezeAllCubes()

	fmt.Println("Waiting 3 seconds before despawning...")
	time.Sleep(3 * time.Second)

	despawnAllCubes()
	fmt.Println("Despawned all cubes.")
}
