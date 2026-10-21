/** Standalone Canvas games. This source is embedded into each exported HTML document. */
function createCanvasMemoryGame({ canvas, onScore, onEnd, isPlaying }) {
  const context = canvas.getContext('2d');
  const columnCount = 4;
  const rowCount = 2;
  const gap = 12;
  const cellWidth = canvas.width / columnCount;
  const cellHeight = canvas.height / rowCount;
  const symbols = ['☀', '✿', '✦', '☁', '☀', '✿', '✦', '☁'];
  for (let index = symbols.length - 1; index > 0; index--) {
    const randomIndex = Math.floor(Math.random() * (index + 1));
    [symbols[index], symbols[randomIndex]] = [symbols[randomIndex], symbols[index]];
  }
  const cards = symbols.map((symbol) => ({
    symbol,
    isOpen: false,
    isMatched: false,
  }));
  let firstIndex = null;
  let selectedIndex = 0;
  let moves = 0;
  let pairs = 0;
  let isLocked = false;
  let isDisposed = false;
  let closeCardsTimeout = null;
  function draw() {
    context.clearRect(0, 0, canvas.width, canvas.height);
    cards.forEach((card, index) => {
      const left = (index % columnCount) * cellWidth + gap / 2;
      const top = Math.floor(index / columnCount) * cellHeight + gap / 2;
      context.globalAlpha = card.isMatched ? 0.45 : 1;
      context.fillStyle = card.isOpen || card.isMatched ? '#fffdf1' : '#d6c366';
      context.beginPath();
      context.roundRect(left, top, cellWidth - gap, cellHeight - gap, 14);
      context.fill();
      if (index === selectedIndex) {
        context.strokeStyle = '#65704e';
        context.lineWidth = 2;
        context.stroke();
      }
      context.fillStyle = '#45533b';
      context.font = '30px Arial';
      context.textAlign = 'center';
      context.textBaseline = 'middle';
      context.fillText(
        card.isOpen || card.isMatched ? card.symbol : '✧',
        left + (cellWidth - gap) / 2,
        top + (cellHeight - gap) / 2,
      );
    });
    context.globalAlpha = 1;
    canvas.setAttribute(
      'aria-label',
      `Найди пару. Ходов ${moves} из 12, найдено пар ${pairs} из 4. Стрелки — выбор карточки, Enter — открыть.`,
    );
  }
  function selectCard(index) {
    if (isDisposed || !isPlaying() || isLocked) {
      return;
    }
    const card = cards[index];
    if (!card || card.isOpen || card.isMatched) {
      return;
    }
    selectedIndex = index;
    card.isOpen = true;
    if (firstIndex === null) {
      firstIndex = index;
      draw();
      return;
    }
    moves++;
    onScore(pairs, moves);
    const previousCard = cards[firstIndex];
    firstIndex = null;
    if (previousCard.symbol === card.symbol) {
      previousCard.isMatched = card.isMatched = true;
      pairs++;
      onScore(pairs, moves);
      draw();
      if (pairs === 4 || moves >= 12) {
        onEnd(pairs === 4);
      }
      return;
    }
    isLocked = true;
    draw();
    closeCardsTimeout = setTimeout(() => {
      if (isDisposed) {
        return;
      }
      previousCard.isOpen = card.isOpen = false;
      isLocked = false;
      draw();
      if (moves >= 12) {
        onEnd(false);
      }
    }, 650);
  }
  canvas.tabIndex = 0;
  canvas.setAttribute('role', 'group');
  canvas.onpointerdown = (event) => {
    event.preventDefault();
    canvas.focus({ preventScroll: true });
    const bounds = canvas.getBoundingClientRect();
    const x = ((event.clientX - bounds.left) / bounds.width) * canvas.width;
    const y = ((event.clientY - bounds.top) / bounds.height) * canvas.height;
    if (x < 0 || y < 0 || x >= canvas.width || y >= canvas.height) {
      return;
    }
    selectCard(Math.floor(y / cellHeight) * columnCount + Math.floor(x / cellWidth));
  };
  canvas.onkeydown = (event) => {
    const offsets = {
      ArrowLeft: -1,
      ArrowRight: 1,
      ArrowUp: -columnCount,
      ArrowDown: columnCount,
    };
    if (event.key in offsets) {
      event.preventDefault();
      selectedIndex = (selectedIndex + offsets[event.key] + cards.length) % cards.length;
      draw();
    } else if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault();
      selectCard(selectedIndex);
    }
  };
  draw();
  return {
    dispose() {
      isDisposed = true;
      clearTimeout(closeCardsTimeout);
      canvas.onpointerdown = canvas.onkeydown = null;
    },
  };
}

function createCanvasClickerGame({ canvas, onScore, onEnd, onTimeChange, isPlaying }) {
  const context = canvas.getContext('2d');
  const centerX = canvas.width / 2;
  const centerY = canvas.height / 2;
  const radius = 78;
  let score = 0;
  let remainingSeconds = 15;
  function draw() {
    context.clearRect(0, 0, canvas.width, canvas.height);
    for (const [circleRadius, color] of [
      [125, '#f6d74d18'],
      [102, '#f6d74d35'],
      [radius, '#f2cf46'],
    ]) {
      context.fillStyle = color;
      context.beginPath();
      context.arc(centerX, centerY, circleRadius, 0, Math.PI * 2);
      context.fill();
    }
    context.fillStyle = '#45533b';
    context.font = '64px Arial';
    context.textAlign = 'center';
    context.textBaseline = 'middle';
    context.fillText('☀', centerX, centerY);
    context.font = '13px Arial';
    context.fillText(`${score} / 30`, centerX, centerY + 110);
    canvas.setAttribute(
      'aria-label',
      `Собрать энергию. Счёт ${score} из 30. Нажмите на солнце, Enter или пробел.`,
    );
  }
  function collectEnergy() {
    if (!isPlaying()) {
      return;
    }
    score++;
    onScore(score);
    draw();
    if (score >= 30) {
      onEnd(true);
    }
  }
  canvas.tabIndex = 0;
  canvas.setAttribute('role', 'button');
  canvas.onpointerdown = (event) => {
    event.preventDefault();
    canvas.focus({ preventScroll: true });
    const bounds = canvas.getBoundingClientRect();
    const x = ((event.clientX - bounds.left) / bounds.width) * canvas.width;
    const y = ((event.clientY - bounds.top) / bounds.height) * canvas.height;
    if (Math.hypot(x - centerX, y - centerY) <= radius) {
      collectEnergy();
    }
  };
  canvas.onkeydown = (event) => {
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault();
      collectEnergy();
    }
  };
  const timer = setInterval(() => {
    if (!isPlaying()) {
      return;
    }
    remainingSeconds--;
    onTimeChange(remainingSeconds);
    if (remainingSeconds <= 0) {
      onEnd(false);
    }
  }, 1000);
  draw();
  return {
    dispose() {
      clearInterval(timer);
      canvas.onpointerdown = canvas.onkeydown = null;
    },
  };
}
