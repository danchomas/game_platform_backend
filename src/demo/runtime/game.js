function runDemoGame(config) {
  const getElement = (id) => document.getElementById(id);
  let isActive = false,
    isPaused = false,
    score = 0,
    timer = null,
    startedAt = 0;
  let snake, direction, nextDirection, food, canvas, context, fallingBalls, tickCount;
  let canvasGame = null;
  getElement('title').textContent = config.title;
  const intros = {
    snake: 'Никуда не спешите. Соберите 5 солнечных яблок.',
    catch: 'Ловите жёлтые шарики. Тёмные лучше пропустить.',
    memory: 'Найдите все пары. У вас есть 12 попыток.',
    click: 'Поймайте свою волну: 30 нажатий за 15 секунд.',
  };
  getElement('intro').textContent = intros[config.kind];
  if (config.kind !== 'snake') {
    document.querySelector('.snake-art').innerHTML =
      config.kind === 'memory'
        ? '<rect x="44" y="10" width="60" height="70" rx="15" fill="#b5c9a8" transform="rotate(-12 70 50)"/><rect x="100" y="24" width="60" height="70" rx="15" fill="#45533b" transform="rotate(12 130 60)"/><circle cx="130" cy="57" r="13" fill="#f7e98a"/>'
        : '<circle cx="100" cy="55" r="35" fill="#e5bd37"/><circle cx="88" cy="49" r="3" fill="#45533b"/><circle cx="111" cy="49" r="3" fill="#45533b"/><path d="M88 65q12 12 24 0" stroke="#45533b" stroke-width="3"/>';
  }
  function notifyHost(type, data) {
    if (window.GameSDK) {
      if (type === 'start') {
        GameSDK.start();
      } else if (type === 'end') {
        GameSDK.end(data.result, data.score);
      }
    }
  }
  function updateScore() {
    getElement('score').textContent = 'Счёт: ' + score;
  }
  function finishRound(win) {
    if (!isActive) {
      return;
    }
    isActive = false;
    clearInterval(timer);
    canvasGame?.dispose();
    getElement('end').style.display = 'flex';
    getElement('result').textContent = win ? 'Отлично получилось!' : 'Попробуем ещё?';
    getElement('result-icon').textContent = win ? '☀' : '↻';
    getElement('final').textContent =
      'Ваш счёт: ' + score + ' · ' + Math.round((Date.now() - startedAt) / 1000) + ' сек.';
    notifyHost('end', {
      result: win ? 'win' : 'lose',
      score,
    });
  }
  function setDirection(directionName) {
    if (!isActive || isPaused || config.kind !== 'snake') {
      return;
    }
    const directionVector = {
      left: {
        x: -1,
        y: 0,
      },
      right: {
        x: 1,
        y: 0,
      },
      up: {
        x: 0,
        y: -1,
      },
      down: {
        x: 0,
        y: 1,
      },
    }[directionName];
    if (
      directionVector &&
      (directionVector.x !== -direction.x || directionVector.y !== -direction.y)
    ) {
      nextDirection = directionVector;
    }
  }
  function placeFood() {
    do {
      food = {
        x: Math.floor(Math.random() * 14),
        y: Math.floor(Math.random() * 14),
      };
    } while (snake.some((segment) => segment.x === food.x && segment.y === food.y));
  }
  function drawSnake() {
    const cellSize = canvas.width / 14;
    context.fillStyle = '#e9ecdd';
    context.fillRect(0, 0, canvas.width, canvas.height);
    context.strokeStyle = '#dde2ce';
    for (let i = 1; i < 14; i++) {
      context.beginPath();
      context.moveTo(i * cellSize, 0);
      context.lineTo(i * cellSize, canvas.height);
      context.moveTo(0, i * cellSize);
      context.lineTo(canvas.width, i * cellSize);
      context.stroke();
    }
    context.fillStyle = '#dfae2c';
    context.beginPath();
    context.arc(
      (food.x + 0.5) * cellSize,
      (food.y + 0.5) * cellSize,
      cellSize * 0.32,
      0,
      Math.PI * 2,
    );
    context.fill();
    snake.forEach((segment, i) => {
      context.fillStyle = i === 0 ? '#303e2c' : '#687c4d';
      context.beginPath();
      context.roundRect(
        segment.x * cellSize + 2,
        segment.y * cellSize + 2,
        cellSize - 4,
        cellSize - 4,
        7,
      );
      context.fill();
      if (!i) {
        context.fillStyle = '#f7e98a';
        context.beginPath();
        context.arc(
          segment.x * cellSize + cellSize * 0.65,
          segment.y * cellSize + cellSize * 0.35,
          2,
          0,
          Math.PI * 2,
        );
        context.fill();
      }
    });
  }
  function advanceSnake() {
    if (isPaused || !isActive) {
      return;
    }
    direction = nextDirection;
    const nextHead = {
      x: snake[0].x + direction.x,
      y: snake[0].y + direction.y,
    };
    const eating = nextHead.x === food.x && nextHead.y === food.y;
    const body = eating ? snake : snake.slice(0, -1);
    if (
      nextHead.x < 0 ||
      nextHead.x >= 14 ||
      nextHead.y < 0 ||
      nextHead.y >= 14 ||
      body.some((segment) => segment.x === nextHead.x && segment.y === nextHead.y)
    ) {
      finishRound(false);
      return;
    }
    snake.unshift(nextHead);
    if (eating) {
      score++;
      updateScore();
      if (score >= 5) {
        finishRound(true);
        return;
      }
      placeFood();
    } else {
      snake.pop();
    }
    drawSnake();
  }
  function advanceFallingBalls() {
    if (isPaused || !isActive) {
      return;
    }
    tickCount++;
    if (tickCount % 18 === 0) {
      fallingBalls.push({
        x: 20 + Math.random() * 260,
        y: -15,
        bad: Math.random() < 0.23,
      });
    }
    fallingBalls.forEach((ball) => {
      ball.y += 2.2 + config.speed * 0.15;
    });
    if (fallingBalls.some((ball) => ball.y > canvas.height + 15 && !ball.bad)) {
      finishRound(false);
      return;
    }
    fallingBalls = fallingBalls.filter((ball) => ball.y < canvas.height + 20);
    context.clearRect(0, 0, canvas.width, canvas.height);
    fallingBalls.forEach((ball) => {
      context.fillStyle = ball.bad ? '#45533b' : '#e8bb37';
      context.beginPath();
      context.arc(ball.x, ball.y, 17, 0, Math.PI * 2);
      context.fill();
    });
    getElement('goal').textContent = 'Цель: 10 шариков';
  }
  function fitCanvasToBoard() {
    if (!canvas) {
      return;
    }
    const board = getElement('board');
    const scale = Math.max(
      0.01,
      Math.min(board.clientWidth / canvas.width, board.clientHeight / canvas.height, 1),
    );
    canvas.style.width = `${canvas.width * scale}px`;
    canvas.style.height = `${canvas.height * scale}px`;
  }
  function createCanvasSurface(width, height) {
    canvas = document.createElement('canvas');
    canvas.width = width;
    canvas.height = height;
    context = canvas.getContext('2d');
    getElement('board').appendChild(canvas);
    fitCanvasToBoard();
  }
  window.addEventListener('resize', fitCanvasToBoard);
  function startRound() {
    clearInterval(timer);
    canvasGame?.dispose();
    canvasGame = null;
    isActive = true;
    isPaused = false;
    score = 0;
    tickCount = 0;
    startedAt = Date.now();
    getElement('pause').textContent = 'Пауза Ⅱ';
    getElement('end').style.display = 'none';
    getElement('cover').style.display = 'none';
    document.querySelector('.stamp').style.display = 'none';
    document.querySelectorAll('.corner').forEach((element) => {
      element.style.display = 'none';
    });
    getElement('game').style.display = 'flex';
    getElement('board').innerHTML = '';
    getElement('controls').style.display = config.kind === 'snake' ? 'flex' : 'none';
    getElement('controls').classList.toggle('large', config.large);
    getElement('pause').style.display = config.kind === 'memory' ? 'none' : 'block';
    updateScore();
    notifyHost('start');
    if (config.kind === 'snake' || config.kind === 'catch') {
      createCanvasSurface(config.kind === 'snake' ? 420 : 300, config.kind === 'snake' ? 420 : 360);
      if (config.kind === 'snake') {
        snake = [
          {
            x: 5,
            y: 7,
          },
          {
            x: 4,
            y: 7,
          },
          {
            x: 3,
            y: 7,
          },
        ];
        direction = nextDirection = {
          x: 1,
          y: 0,
        };
        placeFood();
        drawSnake();
        getElement('goal').textContent = 'Цель: 5 яблок';
        timer = setInterval(advanceSnake, 650 - config.speed * 45);
      } else {
        fallingBalls = [];
        getElement('hint').textContent = 'Нажимайте на жёлтые шарики';
        canvas.onpointerdown = (event) => {
          if (isPaused) {
            return;
          }
          const bounds = canvas.getBoundingClientRect(),
            x = ((event.clientX - bounds.left) / bounds.width) * canvas.width,
            y = ((event.clientY - bounds.top) / bounds.height) * canvas.height;
          const ball = fallingBalls.find((ball) => Math.hypot(ball.x - x, ball.y - y) < 26);
          if (ball) {
            if (ball.bad) {
              finishRound(false);
              return;
            }
            fallingBalls = fallingBalls.filter((otherBall) => otherBall !== ball);
            score++;
            updateScore();
            if (score >= 10) {
              finishRound(true);
            }
          }
        };
        timer = setInterval(advanceFallingBalls, 30);
      }
    }
    if (config.kind === 'memory') {
      createCanvasSurface(360, 200);
      getElement('goal').textContent = 'Ходов: 0 / 12';
      getElement('hint').textContent = 'Нажмите на карточки или выберите стрелками и Enter';
      canvasGame = createCanvasMemoryGame({
        canvas,
        isPlaying: () => isActive && !isPaused,
        onScore: (pairs, moves) => {
          score = pairs;
          updateScore();
          getElement('goal').textContent = `Ходов: ${moves} / 12`;
        },
        onEnd: finishRound,
      });
    }
    if (config.kind === 'click') {
      createCanvasSurface(300, 300);
      getElement('hint').textContent = 'Нажмите на солнце, Enter или пробел';
      getElement('goal').textContent = 'Осталось: 15 с';
      canvasGame = createCanvasClickerGame({
        canvas,
        isPlaying: () => isActive && !isPaused,
        onScore: (value) => {
          score = value;
          updateScore();
        },
        onTimeChange: (seconds) => {
          getElement('goal').textContent = `Осталось: ${seconds} с`;
        },
        onEnd: finishRound,
      });
    }
  }
  getElement('start').onclick = getElement('restart').onclick = startRound;
  getElement('pause').onclick = () => {
    if (!isActive) {
      return;
    }
    isPaused = !isPaused;
    getElement('pause').textContent = isPaused ? 'Продолжить ▷' : 'Пауза Ⅱ';
  };
  document.querySelectorAll('[data-dir]').forEach((button) => {
    button.onpointerdown = (event) => {
      event.preventDefault();
      setDirection(button.dataset.dir);
    };
  });
  window.addEventListener('keydown', (event) => {
    const directionName = {
      ArrowLeft: 'left',
      ArrowRight: 'right',
      ArrowUp: 'up',
      ArrowDown: 'down',
      a: 'left',
      d: 'right',
      w: 'up',
      s: 'down',
    }[event.key];
    if (directionName) {
      event.preventDefault();
      setDirection(directionName);
    }
  });
  let touchStart;
  window.addEventListener(
    'touchstart',
    (event) => {
      touchStart = event.touches[0];
    },
    {
      passive: true,
    },
  );
  window.addEventListener(
    'touchend',
    (event) => {
      if (!touchStart) {
        return;
      }
      const deltaX = event.changedTouches[0].clientX - touchStart.clientX,
        deltaY = event.changedTouches[0].clientY - touchStart.clientY;
      if (Math.max(Math.abs(deltaX), Math.abs(deltaY)) > 15) {
        setDirection(
          Math.abs(deltaX) > Math.abs(deltaY)
            ? deltaX > 0
              ? 'right'
              : 'left'
            : deltaY > 0
              ? 'down'
              : 'up',
        );
      }
      touchStart = null;
    },
    {
      passive: true,
    },
  );
}
