function pvpGame() {
  return {
    state: 'lobby',
    name: '',
    joinId: '',
    roomId: null,
    role: '',
    error: '',
    waitMsg: '',
    pendingToggle: false,
    token: '',   // JWT token for authorized requests

    my: {
      hp: 20, deck: 0, weapon: null, equipped: false,
      durability: [], dungeon: [], healed: false, ran: false
    },
    op: {
      hp: 20, deck: 0, weapon: null, played: null
    },
    gameOutcome: '',
    actionResult: null,

    // ---- persistence ----
    saveToStorage() {
      const data = {
        name: this.name,
        roomId: this.roomId,
        role: this.role,
        state: this.state,
        token: this.token,
      };
      localStorage.setItem('scoundrel_pvp', JSON.stringify(data));
    },
    loadFromStorage() {
      const raw = localStorage.getItem('scoundrel_pvp');
      if (!raw) return false;
      try {
        const data = JSON.parse(raw);
        if (data.roomId && data.role && data.state !== 'lobby') {
          this.name = data.name || '';
          this.roomId = data.roomId;
          this.role = data.role;
          this.token = data.token || '';
          if (data.state === 'waiting') {
            this.state = 'waiting';
            this.waitMsg = `Resuming room ${this.roomId}...`;
            this.waitForStart();
            return true;
          }
          if (data.state === 'playing' || data.state === 'waiting_turn') {
            this.state = 'waiting';
            this.waitMsg = `Reconnecting to room ${this.roomId}...`;
            this.waitForStart();
            return true;
          }
        }
      } catch (e) {}
      return false;
    },
    clearStorage() {
      localStorage.removeItem('scoundrel_pvp');
    },

    // ---- auth helper ----
    async fetchToken(role, roomId = '0') {
      try {
        const res = await fetch('/token', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ role, name: this.name, id: roomId || "0" }),
        });
        if (!res.ok) throw new Error(await res.text());
        const data = await res.json();
        this.token = data.token;
        this.saveToStorage();
      } catch (e) {
        throw new Error('Failed to get token: ' + e.message);
      }
    },

    // ---- helpers ----
    cardImage(cardStr) {
      if (!cardStr) return '';
      const match = cardStr.match(/^(clubs|diamonds|hearts|spades)(.+)$/);
      if (!match) return '';
      const suit = match[1];
      const rank = match[2];
      return `../playing-cards/${suit}_${rank}.png`;
    },
    cardValue(cardStr) {
      if (!cardStr) return 0;
      const rank = cardStr.replace(/^(clubs|diamonds|hearts|spades)/, '');
      const map = { A:14, K:13, Q:12, J:11 };
      return map[rank] || parseInt(rank) || 0;
    },

    // ---- API wrapper (adds auth header) ----
    async request(path, options = {}) {
      const headers = { ...options.headers };
      if (this.token) {
        headers['Authorization'] = 'Bearer ' + this.token;
      }
      const res = await fetch(path, { ...options, headers });
      if (!res.ok) {
        const text = await res.text();
        throw new Error(text || res.statusText);
      }
      return res.json();
    },

    // ---- lobby actions ----
    async createRoom() {
      if (!this.name.trim()) { this.error = 'Enter a name'; return; }
      this.error = '';
      try {
        // get a PLAYER token
        await this.fetchToken('HOST');
        const data = await this.request(`/room/${encodeURIComponent(this.name)}`, { method: 'POST' });
        this.roomId = data.room_id;
        this.role = 'HOST';
        this.state = 'waiting';
        this.waitMsg = `Room ${this.roomId} created. Share this ID with your opponent.`;
        this.saveToStorage();
        await this.waitForStart();
      } catch (e) { this.error = e.message; }
    },

    async joinRoom() {
      if (!this.name.trim()) { this.error = 'Enter a name'; return; }
      const id = parseInt(this.joinId);
      if (isNaN(id)) { this.error = 'Room ID must be a number'; return; }
      this.error = '';
      try {
        await this.fetchToken('GUEST');
        await this.request(`/room/${encodeURIComponent(this.name)}/${id}`, { method: 'PATCH' });
        this.roomId = id;
        this.role = 'GUEST';
        this.state = 'waiting';
        this.waitMsg = `Joined room ${id}. Waiting for host to start...`;
        this.saveToStorage();
        await this.waitForStart();
      } catch (e) { this.error = e.message; }
    },

    async waitForStart() {
      try {
        const result = await this.request(`/game/start/${this.role}/${this.roomId}`);
        this.applyState(result);
        this.saveToStorage();
      } catch (e) {
        this.error = e.message;
        this.state = 'lobby';
        this.clearStorage();
      }
    },

    applyState(result) {
      const gs = result.new_gamestate;
      const op = result.op_gamestate;
      const ar = result.action_result;
      const outcome = result.game_outcome;

      this.my.hp = gs.hp;
      this.my.deck = gs.deck;
      this.my.equipped = gs.equipped;
      this.my.healed = gs.healed;
      this.my.ran = gs.ran;
      this.my.dungeon = (gs.dungeon || []).map(c => c);
      this.my.weapon = gs.weapon || null;

      if (ar && ar.blocked && op.player_card) {
        this.my.durability.push(op.player_card);
        if (this.my.durability.length > 4) this.my.durability.shift();
      } else if (gs.weapon !== this.my.weapon) {
        this.my.durability = [];
      }

      this.op.hp = op.hp;
      this.op.deck = op.deck;
      this.op.weapon = op.weapon || null;
      this.op.played = op.player_card || null;
      this.actionResult = ar;
      this.gameOutcome = outcome || 'GAME_ON';

      if (this.gameOutcome !== 'GAME_ON') {
        this.gameOver = true;                    // <-- new flag
        this.state = 'playing';                  // keep board visible, disable clicks via gameOver
      } else {
        this.state = 'playing';
      }
      this.pendingToggle = false;
      this.saveToStorage();
    }

    async clickCard(idx) {
      if (this.state !== 'playing' || this.gameOver) return;
      this.state = 'waiting_turn';
      try {
        const body = {
          toggle_eapon: this.pendingToggle,
          ran: false,
          clicked_card_index: idx
        };
        const result = await this.request(`/game/${this.role}/${this.roomId}`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(body)
        });
        this.applyState(result);
      } catch (e) {
        this.error = e.message;
        if (e.message.includes('Gone')) {
          this.state = 'result';
          this.gameOutcome = 'TIE';
        } else {
          this.state = 'playing';
        }
      }
    },

    async runAway() {
      if (this.state !== 'playing' || this.gameOver || this.my.ran || this.my.dungeon.length < 4) return;
      this.state = 'waiting_turn';
      try {
        const body = {
          toggle_eapon: false,
          ran: true,
          clicked_card_index: 0
        };
        const result = await this.request(`/game/${this.role}/${this.roomId}`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(body)
        });
        this.applyState(result);
      } catch (e) {
        this.error = e.message;
        this.state = 'playing';
      }
    },

    toggleWeapon() {
      if (this.state !== 'playing' || this.gameOver) return;
      this.pendingToggle = !this.pendingToggle;
      this.my.equipped = !this.my.equipped;
    },

    async leaveGame() {
      try {
        await this.request(`/game/${this.role}/${this.roomId}`, { method: 'DELETE' });
      } catch (e) {}
      this.resetToLobby();
    },

    resetToLobby() {
      this.state = 'lobby';
      this.roomId = null;
      this.role = '';
      this.error = '';
      this.pendingToggle = false;
      this.token = '';
      this.my = { hp:20, deck:0, weapon:null, equipped:false, durability:[], dungeon:[], healed:false, ran:false };
      this.op = { hp:20, deck:0, weapon:null, played:null };
      this.gameOutcome = '';
      this.clearStorage();
    },

    get outcomeText() {
      return this.gameOutcome === 'YOU_WIN' ? '🏆 You Win!' :
             this.gameOutcome === 'YOU_LOSE' ? '💀 You Lose' :
             this.gameOutcome === 'TIE' ? '🤝 Tie' : 'Game Over';
    },

    init() {
      if (!this.loadFromStorage()) {
        this.state = 'lobby';
      }
    }
  };
}
