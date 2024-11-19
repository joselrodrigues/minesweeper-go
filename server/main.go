package main

import (
	"context"
	"fmt"
	"log"
	g "minesweeper/game"
	pb "minesweeper/proto"
	"net"
	"os"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"google.golang.org/grpc"
)

type gameServer struct {
	pb.UnimplementedMinesweeperServer
	game *g.Game
	mu   sync.Mutex
}

func startGRPCServer(game *g.Game) {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		fmt.Errorf("failed to listen: %v", err)
		os.Exit(1)
	}

	s := grpc.NewServer()
	pb.RegisterMinesweeperServer(s, &gameServer{game: game})

	log.Printf("Starting gRPC server on :50051")
	if err := s.Serve(lis); err != nil {
		fmt.Errorf("failed to serve: %v", err)
		os.Exit(1)
	}
}

func startEbitenWindow(game *g.Game) {
	ebiten.SetWindowSize(g.DefaultWindowWidth, g.DefaultWindowHeight)
	ebiten.SetWindowTitle("MineSweeper")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)

	// game.AudioManager.LoadSound("totalmenchi", "assets/sounds/totalmenchi.mp3")

	if err := ebiten.RunGame(game); err != nil {
		fmt.Errorf("ebiten error: %v", err)
		os.Exit(1)
	}
}

func main() {
	game, err := g.NewGame(g.Medium)
	if err != nil {
		log.Fatal(err)
	}

	go startGRPCServer(game)

	startEbitenWindow(game)
}

func (s *gameServer) MakeMove(ctx context.Context, move *pb.Move) (*pb.GameState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var action g.ActionEvent
	switch move.Action {
	case 0:
		action = g.RevealCell
	case 1:
		action = g.ToggleFlag
	default:
		return nil, fmt.Errorf("invalid action: %d", move.Action)
	}

	coord := g.Coordinates{X: int(move.X), Y: int(move.Y)}
	posx, posy := s.game.BoardToScreen(coord)
	pos := g.Coordinates{X: int(posx), Y: int(posy)}

	oldCellState := s.game.Board[coord]

	err := s.game.HandleInput(pos, action)
	modelState := s.game.ModelState()
	reward := s.game.CalculateModelReward(oldCellState, action)

	protoRows := make([]*pb.Row, len(modelState))
	for i, row := range modelState {
		protoRows[i] = &pb.Row{
			Cell: row,
		}
	}

	if err != nil {
		return nil, err
	}

	return &pb.GameState{
		Board:  protoRows,
		Reward: int32(reward),
		State:  int32(s.game.State),
	}, nil
}

func (s *gameServer) Reset(ctx context.Context, _ *pb.Empty) (*pb.GameState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.game.Restart()

	modelState := s.game.ModelState()
	protoRows := make([]*pb.Row, len(modelState))
	for i, row := range modelState {
		protoRows[i] = &pb.Row{
			Cell: row,
		}
	}

	return &pb.GameState{
		Board:  protoRows,
		Reward: 0,
		State:  int32(s.game.State),
	}, nil
}
