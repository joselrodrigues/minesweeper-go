package main

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/png"
	"io"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/mp3"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

const (
	boardOffsetX        = 12
	boardOffsetY        = 55
	cellSize            = 16
	sampleRate          = 48000
	defaultWindowWidth  = 1080
	defaultWindowHeight = 720
	spreadFactor        = 2.0 // Mayor número = más dispersión
	centerBias          = 0.5 // 0.5 = centrado, ajustar para desplazar el centro
)

var (
	ErrOutOfBounds       = errors.New("position is out of bounds")
	ErrInvalidDifficulty = errors.New("invalid game difficulty")
	ErrAssetNotFound     = errors.New("game asset not found")
)

type GameState int

const (
	Playing GameState = iota
	Won
	Lost
)

type DificultyLevel int

const (
	Easy DificultyLevel = iota
	Medium
	Hard
)

var difficultyLevels = map[DificultyLevel]GameDifficulty{
	Easy: {
		GridDimensions: GridDimensions{Cols: 9, Rows: 9},
		NumberOfMines:  10,
	},

	Medium: {
		GridDimensions: GridDimensions{Cols: 16, Rows: 16},
		NumberOfMines:  40,
	},

	Hard: {
		GridDimensions: GridDimensions{Cols: 30, Rows: 16},
		NumberOfMines:  99,
	},
}

type AudioManager struct {
	context *audio.Context
	sounds  map[string]*audio.Player
}

var PositionNeighbors = []Coordinates{
	{X: -1, Y: -1},
	{X: -1, Y: 0},
	{X: -1, Y: 1},
	{X: 0, Y: -1},
	{X: 0, Y: 1},
	{X: 1, Y: -1},
	{X: 1, Y: 0},
	{X: 1, Y: 1},
}

type GameStatistics struct {
	StartTime      time.Time
	TimeElapsed    time.Duration
	Clicks         int
	FlagsAvailable int
}

type Game struct {
	Board         map[Coordinates]CellState
	MinePositions map[Coordinates]bool
	AudioManager  *AudioManager
	Statistics    *GameStatistics
	FirstClick    *Coordinates
	Sprite        Sprite
	Difficulty    GameDifficulty
	State         GameState
}

type Coordinates struct {
	X, Y int
}

// TODO: keep CellState invariants (keep the struct consistent)
// example: if isMine is true, minesAround should be 0
// example: if isRevealed is true, isFlag should be false
// example: if isFlag is true, isRevealed should be false
type CellState struct {
	minesAround   int
	isMine        bool
	isFlag        bool
	isRevealed    bool
	isMineClicked bool
}

type ClickEvent struct {
	Position Coordinates
	IsFirst  bool
}

type Sprite struct {
	Image map[string]*ebiten.Image
}

type GridDimensions struct {
	Cols int
	Rows int
}

type GameDifficulty struct {
	NumberOfMines  int
	GridDimensions GridDimensions
}

func NewAudioManager() (*AudioManager, error) {
	context := audio.NewContext(sampleRate)
	return &AudioManager{
		context: context,
		sounds:  make(map[string]*audio.Player),
	}, nil
}

func (am *AudioManager) LoadSound(name string, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open audio file: %w", err)
	}

	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("failed to read audio file: %w", err)
	}

	reader := bytes.NewReader(data)
	decoded, err := mp3.DecodeWithSampleRate(sampleRate, reader)
	if err != nil {
		return fmt.Errorf("failed to decode audio file: %w", err)
	}

	player, err := am.context.NewPlayer(decoded)
	if err != nil {
		return fmt.Errorf("failed to create audio player: %w", err)
	}

	am.sounds[name] = player
	return nil
}

func (am *AudioManager) PlaySound(name string) error {
	player, ok := am.sounds[name]
	if !ok {
		return ErrAssetNotFound
	}

	player.Rewind()
	player.Play()
	return nil
}

func NewGame(level DificultyLevel) (*Game, error) {
	difficulty, ok := difficultyLevels[level]
	if !ok {
		return nil, ErrInvalidDifficulty
	}

	// TODO: change name of the function to NewSprite
	sprite, err := LoadSprite()
	if err != nil {
		return nil, fmt.Errorf("failed to load sprites: %w", err)
	}

	audioManager, err := NewAudioManager()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize audio: %w", err)
	}

	game := &Game{
		Board:         make(map[Coordinates]CellState),
		MinePositions: make(map[Coordinates]bool),
		Statistics:    &GameStatistics{StartTime: time.Now(), FlagsAvailable: difficulty.NumberOfMines},
		Difficulty:    difficulty,
		AudioManager:  audioManager,
		State:         Playing,
		Sprite:        sprite,
		FirstClick:    nil,
	}

	// TODO: mabye shoudl handle error
	game.CreateBoard()

	return game, nil
}

func (g *Game) InitializeBoardState() {
	g.GenerateMinePositions()
	for pos := range g.Board {
		g.CalculateMinesAround(pos)
	}
}

func (g *Game) CreateBoard() {
	grid := g.Difficulty.GridDimensions
	for x := 0; x < grid.Cols; x++ {
		for y := 0; y < grid.Rows; y++ {
			pos := Coordinates{X: x, Y: y}
			g.Board[pos] = CellState{isMine: false, isFlag: false, isRevealed: false, minesAround: 0}
		}
	}
}

func (g *Game) RevealAllMines() {
	for minesPos := range g.MinePositions {
		cellState := g.Board[minesPos]
		if !cellState.isFlag && !cellState.isRevealed {
			cellState.isRevealed = true
			g.Board[minesPos] = cellState
		}
	}
}

func (g *Game) HandleMineClicked(pos Coordinates) {
	cellState := g.Board[pos]
	cellState.isMineClicked = true
	cellState.isRevealed = true
	g.Board[pos] = cellState
	g.State = Lost
	g.RevealAllMines()
}

// TODO: Implement Restart
func (g *Game) Restart() {
	g.Board = make(map[Coordinates]CellState)
	g.MinePositions = make(map[Coordinates]bool)
	g.State = Playing
	g.Statistics = &GameStatistics{StartTime: time.Now(), FlagsAvailable: g.Difficulty.NumberOfMines}
	g.FirstClick = nil

	g.CreateBoard()

	for _, player := range g.AudioManager.sounds {
		player.Rewind()
		player.Pause()
	}
}

func (g *Game) RevealCell(pos Coordinates) error {
	// TODO: maybe this is no necessary here because
	// this should be handle in the Update method
	if g.State != Playing {
		return nil
	}

	// TODO: this should be handle with an error inside of the function
	if g.isOutOfBounds(pos) {
		return nil
	}

	cellState := g.Board[pos]

	if cellState.isRevealed || cellState.isFlag {
		return nil
	}

	if cellState.isMine {
		g.HandleMineClicked(pos)
		return nil
	}

	g.RevealCellChain(pos)

	return nil
}

func (g *Game) RevealCellChain(position Coordinates) {
	if g.isOutOfBounds(position) {
		return
	}

	cellState := g.Board[position]

	if cellState.isRevealed || cellState.isFlag {
		return
	}

	if cellState.isMine {
		cellState.isMineClicked = true
		g.Board[position] = cellState
		g.RevealAllMines()
		return
	}

	if cellState.minesAround > 0 {
		cellState.isRevealed = true
		g.Board[position] = cellState
		return
	}

	for _, neighbor := range PositionNeighbors {
		neighborPos := Coordinates{X: position.X + neighbor.X, Y: position.Y + neighbor.Y}

		if g.isOutOfBounds(neighborPos) {
			continue
		}

		cellState.isRevealed = true
		g.Board[position] = cellState
		if cellState.minesAround == 0 && !g.Board[neighborPos].isMine {
			g.RevealCellChain(neighborPos)
		}
	}
}

func (g *Game) CalculateMinesAround(position Coordinates) {
	mines := g.MinePositions

	if mines[position] {
		g.Board[position] = CellState{isMine: true, isFlag: false, isRevealed: false, minesAround: 0}

		for _, neighbor := range PositionNeighbors {
			neighborPos := Coordinates{X: position.X + neighbor.X, Y: position.Y + neighbor.Y}

			if g.isOutOfBounds(neighborPos) {
				continue
			}

			if _, exists := g.Board[neighborPos]; !exists {
				g.Board[neighborPos] = CellState{isMine: false, isFlag: false, isRevealed: false, minesAround: 0}
			}

			if _, exists := mines[neighborPos]; !exists {
				cellState := g.Board[neighborPos]
				cellState.minesAround++
				g.Board[neighborPos] = cellState
			}
		}
	}

	if _, exists := g.Board[position]; !exists {
		g.Board[position] = CellState{isMine: false, isFlag: false, isRevealed: false, minesAround: 0}
		return
	}
}

func (g *Game) GenerateMinePositions() {
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))

	// TODO: maybe is not necessary to store this in a variable
	grid := g.Difficulty.GridDimensions
	numberOfMines := g.Difficulty.NumberOfMines
	mines := g.MinePositions

	for len(mines) < numberOfMines {
		x := int(rnd.NormFloat64()*float64(grid.Cols)/spreadFactor +
			float64(grid.Cols)*centerBias)
		y := int(rnd.NormFloat64()*float64(grid.Rows)/spreadFactor +
			float64(grid.Rows)*centerBias)
		pos := Coordinates{X: x, Y: y}
		isNotFirstClick := g.FirstClick == nil || pos != *g.FirstClick
		if !g.isOutOfBounds(pos) && !mines[pos] && isNotFirstClick {
			mines[pos] = true
		}
	}
}

// TODO: maybe should be call ValidatePosition
func (g *Game) isOutOfBounds(position Coordinates) bool {
	return position.X < 0 || position.Y < 0 || position.X >= g.Difficulty.GridDimensions.Cols || position.Y >= g.Difficulty.GridDimensions.Rows
}

func (g *Game) ValidBoardPosition(cursorX, cursorY int) (Coordinates, bool) {
	cellX := (cursorX - boardOffsetX) / cellSize
	cellY := (cursorY - boardOffsetY) / cellSize
	pos := Coordinates{X: cellX, Y: cellY}

	cellRect := image.Rect(
		boardOffsetX+cellX*cellSize,
		boardOffsetY+cellY*cellSize,
		boardOffsetX+(cellX+1)*cellSize,
		boardOffsetY+(cellY+1)*cellSize,
	)

	if !image.Pt(cursorX, cursorY).In(cellRect) || g.isOutOfBounds(pos) {
		return Coordinates{}, false
	}

	return pos, true
}

func (g *Game) ScreenToBoard(screenX, screenY int) Coordinates {
	boardX := (screenX - boardOffsetX) / cellSize
	boardY := (screenY - boardOffsetY) / cellSize
	return Coordinates{X: boardX, Y: boardY}
}

func (g *Game) Update() error {
	if g.State != Playing {
		return nil
	}

	x, y := ebiten.CursorPosition()

	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		pos, ok := g.ValidBoardPosition(x, y)

		if !ok {
			return nil
		}

		if g.FirstClick == nil {
			g.FirstClick = &pos
			g.InitializeBoardState()
		}

		cellState := g.Board[pos]
		if cellState.minesAround == 0 && !cellState.isMine && !cellState.isRevealed && !cellState.isFlag {
			g.AudioManager.PlaySound("totalmenchi")
		}

		g.RevealCell(pos)
		g.CheckVictory()
	}
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight) {

		position, ok := g.ValidBoardPosition(x, y)

		if !ok {
			return nil
		}
		g.ToggleFlag(position)

	}
	return nil
}

func (g *Game) ToggleFlag(pos Coordinates) {
	cellState := g.Board[pos]
	if cellState.isRevealed {
		return
	}

	if !cellState.isFlag && g.Statistics.FlagsAvailable == 0 {
		return
	}

	cellState.isFlag = !cellState.isFlag
	g.Board[pos] = cellState

	if cellState.isFlag {
		g.Statistics.FlagsAvailable--
	} else {
		g.Statistics.FlagsAvailable++
	}
}

func (g *Game) CheckVictory() bool {
	if g.State != Playing {
		return false
	}
	for _, cellState := range g.Board {
		if cellState.isMine {
			if !cellState.isFlag {
				return false
			}
		} else {
			if !cellState.isRevealed {
				return false
			}
		}
	}
	g.State = Won
	// TODO: should collect some statistics
	return true
}

func (g *Game) BoardToScreen(boardPos Coordinates) (float64, float64) {
	// X: borde izquierdo + (posición * tamaño de celda)
	screenX := float64(boardOffsetX + (boardPos.X * cellSize))

	// Y: borde superior + (posición * tamaño de celda)
	screenY := float64(boardOffsetY + (boardPos.Y * cellSize))

	return screenX, screenY
}

func (g *Game) RenderBoard(screen *ebiten.Image) {
	for boardPosition, cellState := range g.Board {
		var baseSpriteCell *ebiten.Image
		opts := &ebiten.DrawImageOptions{}

		screenX, screenY := g.BoardToScreen(boardPosition)

		opts.GeoM.Translate(screenX, screenY)

		switch {
		case cellState.isFlag && !cellState.isRevealed:
			baseSpriteCell = g.Sprite.Image["flag"]
		case !cellState.isRevealed:
			baseSpriteCell = g.Sprite.Image["hidden"]
		case cellState.isMineClicked:
			baseSpriteCell = g.Sprite.Image["mineClicked"]
		case cellState.isMine:
			baseSpriteCell = g.Sprite.Image["mine"]
		case cellState.minesAround > 0:
			baseSpriteCell = g.Sprite.Image[fmt.Sprintf("number_%d", cellState.minesAround)]
		default:
			baseSpriteCell = g.Sprite.Image["empty"]
		}

		screen.DrawImage(baseSpriteCell, opts)

	}
}

func (g *Game) RenderUI(screen *ebiten.Image) {
	baseSpriteCell := g.Sprite.Image["mineClicked"]
	opts := &ebiten.DrawImageOptions{}

	tx := float64(cellSize)
	ty := float64(0)

	opts.GeoM.Translate(tx, ty)

	screen.DrawImage(baseSpriteCell, opts)
}

func (g *Game) Draw(screen *ebiten.Image) {
	g.RenderBoard(screen)
	// g.RenderUI(screen)
}

func (g *Game) CalculateScreenDimensions() (width, height int) {
	// Ancho: borde izquierdo + ancho del tablero + borde derecho
	width = boardOffsetX + (g.Difficulty.GridDimensions.Cols * cellSize) + boardOffsetX

	// Alto: borde superior + alto del tablero + borde inferior
	height = boardOffsetY + (g.Difficulty.GridDimensions.Rows * cellSize) + boardOffsetX // usamos boardOffsetX porque el borde inferior es 12

	return width, height
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return g.CalculateScreenDimensions()
}

func LoadSprite() (Sprite, error) {
	spriteSheet, _, err := ebitenutil.NewImageFromFile("assets/sprites/board.png")

	images := map[string]*ebiten.Image{
		"hidden":      spriteSheet.SubImage(image.Rect(0, 0, cellSize, cellSize)).(*ebiten.Image),
		"flag":        spriteSheet.SubImage(image.Rect(cellSize*2, 0, cellSize*3, cellSize)).(*ebiten.Image),
		"mineClicked": spriteSheet.SubImage(image.Rect(cellSize*6, 0, cellSize*7, cellSize)).(*ebiten.Image),
		"mine":        spriteSheet.SubImage(image.Rect(cellSize*5, 0, cellSize*6, cellSize)).(*ebiten.Image),
		"empty":       spriteSheet.SubImage(image.Rect(cellSize, 0, cellSize*2, cellSize)).(*ebiten.Image),
	}

	for spriteNumb := 0; spriteNumb < 8; spriteNumb++ {
		images[fmt.Sprintf("number_%d", spriteNumb+1)] = spriteSheet.SubImage(image.Rect(cellSize*spriteNumb, cellSize, cellSize*(spriteNumb+1), cellSize*2)).(*ebiten.Image)
	}

	return Sprite{Image: images}, err
}

func main() {
	ebiten.SetWindowSize(defaultWindowWidth, defaultWindowHeight)
	ebiten.SetWindowTitle("MineSweeper")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)

	game, err := NewGame(Medium)
	if err != nil {
		log.Fatal(err)
	}

	game.AudioManager.LoadSound("totalmenchi", "./assets/sounds/totalmenchi.mp3")

	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
