# Minesweeper with Go and Ebitengine

Welcome to Minesweeper!
This is a simple Minesweeper project developed in Go using the Ebitengine graphics library. Minesweeper is a classic game where the goal is to uncover all cells in a grid that do not contain mines, using numerical clues that indicate how many mines are adjacent to a cell.

## Features

- Intuitive and easy-to-play graphical interface.
- Classic Minesweeper functionality.
- Three difficulty levels: Easy, Medium, Hard.
- Developed in Go with Ebitengine, a lightweight engine for 2D games.

## Requirements

- Go 1.18 or higher.
- [Ebitengine](https://ebitengine.org/): Graphics library for Go.

## Installation

1. **Clone the repository**

   ```bash
   git clone https://github.com:joselrodrigues/minesweeper.git
   cd minesweeper
   ```

2. **Install dependencies**

   Make sure you have Go installed. You can install the Ebitengine library with the following command:

   ```bash
   go get github.com/hajimehoshi/ebiten/v2
   ```

3. **Run the project**

   Once the dependencies are installed, you can run the game with the command:

   ```bash
   go run main.go
   ```
