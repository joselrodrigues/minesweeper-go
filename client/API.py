import os
import sys
import grpc

root_dir = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
sys.path.append(root_dir)
sys.path.append(os.path.join(root_dir, "proto"))

from proto.minesweeper_pb2 import Move, Empty
from proto.minesweeper_pb2_grpc import MinesweeperStub

channel = grpc.insecure_channel("localhost:50051")


class MinesweeperAPI:
    def __init__(self):
        self.stub = MinesweeperStub(channel)

    def protobuf_board_to_array(self, proto_board):
        return [list(board.Cell) for board in proto_board]

    def make_move(self, x, y, action):
        response = self.stub.MakeMove(Move(x=x, y=y, action=action))
        board = self.protobuf_board_to_array(response.Board)
        return board, response.Reward, response.State

    def reset(self):
        response = self.stub.Reset(Empty())
        board = self.protobuf_board_to_array(response.Board)
        return board
