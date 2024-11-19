package minesweeper

import (
	"time"
)

type (
	Action     int
	CellState  int
	GameState  int
	Difficulty int
)

const (
	Hidden CellState = iota
	Revealed
	Flagged
)

const (
	Reveal Action = iota
	Flag
)

const (
	Playing GameState = iota
	Won
	Lost
)

const (
	Easy Difficulty = iota
	Medium
	Hard
)

var difficultySettings = map[Difficulty]GameSettings{
	Easy: {
		BoardDimension: BoardDimension{Cols: 9, Rows: 9},
		NumberOfMines:  10,
	},

	Medium: {
		BoardDimension: BoardDimension{Cols: 16, Rows: 16},
		NumberOfMines:  40,
	},

	Hard: {
		BoardDimension: BoardDimension{Cols: 30, Rows: 16},
		NumberOfMines:  99,
	},
}

type Coordinates struct {
	X, Y int
}

type BoardDimension struct {
	Rows, Cols int
}

type GameSettings struct {
	NumberOfMines  int
	BoardDimension BoardDimension
}

type Cell struct {
	State       CellState
	IsMine      bool
	MinesAround int
}

type Statistics struct {
	StartTime      time.Time
	TimeElapsed    time.Duration
	Clicks         int
	FlagsAvailable int
}

type Game struct {
	Board      map[Coordinates]Cell
	State      GameState
	Difficulty GameSettings
	Statistics *Statistics
}

func (g *Game) createInitialBoard() {
	for Row := 0; Row < g.Difficulty.BoardDimension.Rows; Row++ {
		for Col := 0; Col < g.Difficulty.BoardDimension.Cols; Col++ {
			g.Board[Coordinates{X: Row, Y: Col}] = Cell{State: Hidden}
		}
	}
}

func NewGame(level Difficulty) (*Game, error) {
	difficulty := difficultySettings[level]

	// if !ok {
	// 	return nil, ErrInvalidDifficulty
	// }

	// sprite, err := LoadSprite()
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to load sprites: %w", err)
	// }

	// audioManager, err := NewAudioManager()
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to initialize audio: %w", err)
	// }

	game := &Game{
		Board: make(map[Coordinates]Cell),
		// MinePositions: make(map[Coordinates]bool),
		Statistics: &Statistics{StartTime: time.Now(), FlagsAvailable: difficulty.NumberOfMines},
		Difficulty: difficulty,
		// AudioManager:  audioManager,
		State: Playing,
		// Sprite:     sprite,
		FirstClick: nil,
	}

	game.createInitialBoard()

	return game, nil
}
