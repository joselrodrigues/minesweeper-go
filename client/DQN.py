import math
import random
import matplotlib
import matplotlib.pyplot as plt
from collections import namedtuple, deque
from itertools import count

import torch
import torch.nn as nn
import torch.optim as optim
import torch.nn.functional as F


from .API import MinesweeperAPI

is_ipython = "inline" in matplotlib.get_backend()
if is_ipython:
    from IPython import display

plt.ion()


device = torch.device(
    "cuda"
    if torch.cuda.is_available()
    else "mps"
    if torch.backends.mps.is_available()
    else "cpu"
)


Transition = namedtuple("Transition", ("state", "action", "next_state", "reward"))


class ReplayMemory(object):
    def __init__(self, capacity):
        self.memory = deque([], maxlen=capacity)

    def push(self, *args):
        """Save a transition"""
        self.memory.append(Transition(*args))

    def sample(self, batch_size):
        return random.sample(self.memory, batch_size)

    def __len__(self):
        return len(self.memory)


class DQN(nn.Module):
    def __init__(self, n_actions=2, game_rows=16, game_cols=16):
        super(DQN, self).__init__()
        # TODO: las columnas y filas del juego deben ser variables

        self.game_rows = game_rows
        self.game_cols = game_cols
        self.conv_layers = nn.Sequential(
            # Entrada: (batch, 1, 16, 16)
            # Salida:  (batch, 64, 16, 16)
            nn.Conv2d(
                in_channels=1,
                out_channels=64,
                kernel_size=3,
                padding=1,
            ),
            nn.BatchNorm2d(64),
            nn.ReLU(),
            # Entrada: (batch, 64, 16, 16)
            # Salida:  (batch, 128, 16, 16)
            nn.Conv2d(in_channels=64, out_channels=128, kernel_size=3, padding=1),
            nn.BatchNorm2d(128),
            nn.ReLU(),
            # Entrada: (batch, 128, 16, 16)
            # Salida:  (batch, 128, 16, 16)
            nn.Conv2d(in_channels=128, out_channels=128, kernel_size=3, padding=1),
            nn.BatchNorm2d(128),
            nn.ReLU(),
        )

        flat_features = self.game_cols * self.game_rows * 128
        n_actions = self.game_cols * self.game_rows * 2

        self.fc_layers = nn.Sequential(
            nn.Linear(flat_features, 512),
            nn.BatchNorm1d(512),
            nn.ReLU(),
            nn.Dropout(0.3),
            nn.Linear(512, 512),
            nn.BatchNorm1d(512),
            nn.ReLU(),
            nn.Dropout(0.3),
            nn.Linear(512, n_actions),
        )

    def forward(self, x):
        batch_size = x.size(0)

        x = self.conv_layers(x)
        x = x.view(batch_size, -1)
        x = self.fc_layers(x)

        return x.view(batch_size, -1, 2)


class DQNAgent:
    def __init__(self, game_rows=16, game_cols=16):
        self.device = device  # Usamos el device que definimos anteriormente
        self.game_rows = game_rows
        self.game_cols = game_cols

        self.learning_rate = 0.001
        self.weight_decay = 1e-4
        self.batch_size = 32
        self.gamma = 0.99  # Factor de descuento para recompensas futuras
        self.epsilon = 1.0  # Probabilidad de exploración inicial
        self.epsilon_min = 0.01  # Mínima probabilidad de exploración
        self.epsilon_decay = 0.995  # Tasa de decaimiento de epsilon

        # Creamos dos redes: la principal (policy) y la target
        self.policy_net = DQN(game_rows=self.game_rows, game_cols=self.game_cols).to(
            device
        )
        self.target_net = DQN(game_rows=self.game_rows, game_cols=self.game_cols).to(
            device
        )
        # Copiamos los pesos iniciales de policy a target
        self.target_net.load_state_dict(self.policy_net.state_dict())

        # Optimizador para el entrenamiento
        self.optimizer = optim.Adam(
            self.policy_net.parameters(),
            lr=self.learning_rate,
            weight_decay=self.weight_decay,
        )

        # Memoria para experiencia replay
        self.memory = ReplayMemory(10000)

    def flat_to_coord(self, pos):
        """Convierte posición plana a coordenadas (x,y)"""
        if pos >= self.game_rows * self.game_cols:
            raise ValueError(f"Posición {pos} fuera de rango")
        row = pos // self.game_cols
        col = pos % self.game_cols
        return (col, row)  # x,y para el tablero

    def coord_to_flat(self, x, y):
        """Convierte coordenadas (x,y) a posición plana"""
        if x >= self.game_cols or y >= self.game_rows:
            raise ValueError(f"Coordenadas ({x},{y}) fuera de rango")
        return y * self.game_cols + x

    def select_action(self, state):
        if random.random() < self.epsilon:
            # Exploración: acción aleatoria
            pos = random.randint(0, self.game_rows * self.game_cols - 1)
            action = random.randint(0, 1)
            x, y = self.flat_to_coord(pos)
            return (x, y), action

        # Explotación: usar la red
        with torch.no_grad():
            state = torch.FloatTensor(state).unsqueeze(0).unsqueeze(0).to(self.device)
            q_values = self.policy_net(state).squeeze(0)  # Shape: (256, 2)

            # Encontrar la mejor acción para cada posición
            best_value, best_action = q_values.max(
                dim=1
            )  # mejor acción para cada posición
            best_pos = best_value.argmax()  # mejor posición
            best_action = best_action[best_pos]  # acción para la mejor posición

            x, y = self.flat_to_coord(best_pos.item())
            return (x, y), best_action.item()

    def optimize_model(self):
        if len(self.memory) < self.batch_size:
            return
        transitions = self.memory.sample(self.batch_size)
        batch = Transition(*zip(*transitions))

        # Procesar las acciones
        actions = []
        for action in batch.action:
            coords, action_type = action
            x, y = coords
            pos = self.coord_to_flat(x, y)
            flat_action = pos * 2 + action_type
            actions.append(flat_action)

        non_final_mask = torch.tensor(
            tuple(map(lambda s: s is not None, batch.next_state)),
            device=self.device,
            dtype=torch.bool,
        )

        non_final_next_states = torch.cat(
            [
                torch.FloatTensor(s).unsqueeze(0).unsqueeze(0)
                for s in batch.next_state
                if s is not None
            ]
        ).to(self.device)

        state_batch = torch.cat(
            [torch.FloatTensor(s).unsqueeze(0).unsqueeze(0) for s in batch.state]
        ).to(self.device)

        action_batch = torch.tensor(actions).to(self.device)
        reward_batch = torch.tensor(batch.reward).to(self.device)

        # Calcular Q(s_t, a)
        all_q_values = self.policy_net(state_batch)
        state_action_values = all_q_values.view(self.batch_size, -1).gather(
            1, action_batch.unsqueeze(1)
        )

        # Calcular V(s_{t+1}) para todos los next states
        next_state_values = torch.zeros(self.batch_size, device=self.device)

        if len(non_final_next_states) > 0:
            with torch.no_grad():
                # Obtener Q-values para los next states
                next_q_values = self.target_net(non_final_next_states)
                # Reshape a (batch_size, num_positions * num_actions)
                next_q_values = next_q_values.view(next_q_values.size(0), -1)
                # Obtener el máximo Q-value para cada estado
                next_state_values[non_final_mask] = next_q_values.max(1)[0]

        # Calcular expected Q values
        expected_state_action_values = (next_state_values * self.gamma) + reward_batch

        # Calcular pérdida
        loss = F.smooth_l1_loss(
            state_action_values, expected_state_action_values.unsqueeze(1)
        )

        self.optimizer.zero_grad()
        loss.backward()
        self.optimizer.step()


# Clase ambiente que interactúa con la API
class DQNEnv:
    def __init__(self):
        self.api = MinesweeperAPI()

    def step(self, action):
        x, y = action[0]  # Las coordenadas
        action_type = action[1]  # revelar/bandera

        # Obtener respuesta de la API
        next_state, reward, game_state = self.api.make_move(x, y, action_type)

        # Determinar si el episodio terminó
        done = game_state != 0  # 0 es Playing

        # Guardar la experiencia
        return next_state, reward, done, {}

    def reset(self):
        """Reinicia el entorno y retorna el estado inicial"""
        initial_state = self.api.reset()
        return initial_state


def train(num_episodes=50000):
    env = DQNEnv()
    agent = DQNAgent()

    for episode in range(num_episodes):
        state = env.reset()
        episode_reward = 0
        done = False

        while not done:
            # Seleccionar acción
            action = agent.select_action(state)

            # Ejecutar acción y obtener respuesta de la API
            next_state, reward, done, _ = env.step(action)

            # Guardar experiencia en la memoria
            agent.memory.push(state, action, next_state, reward)

            # Entrenar el modelo
            agent.optimize_model()

            state = next_state
            episode_reward += reward
