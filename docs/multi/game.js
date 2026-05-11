document.addEventListener('alpine:init', () => {
const cards = ['A','2','3','4','5','6','7','8','9','10','J','Q','K'];
const suits = ['clubs','diamonds','hearts','spades'];
const valid_cards = cards.flatMap( card => suits.map(suit => ({ card, suit }))).filter(
			el=> {
				const legal = !(isNaN(Number(el.card)) && (el.suit == 'hearts' || el.suit=='diamonds'));
				if(!legal) console.log(el);
				return legal
			}
		);

const saved = localStorage.getItem('gameState');
console.log("Gamestate in localStorage:",saved);
function new_gamestate() { 
	return {
		// gamestate
		deck: valid_cards.toSorted(() => Math.random() - 0.5),
		weapon: null,
		durability: [],
		equipped: true,
		ran: false,
		healed: false,
		dungeon: [],
		hp:20,

	};
}
var gamestate = saved? JSON.parse(saved): new_gamestate()
Alpine.store('game', {
		...gamestate,
		// game functions 
		save(){
			console.log("Game saved");
			localStorage.setItem('gameState', JSON.stringify({
				deck:this.deck,
				weapon:this.weapon,
				durability:this.durability,
				equipped:this.equipped,
				ran:this.ran,
				healed:this.healed,
				dungeon:this.dungeon,
				hp:this.hp,
			}))
		},
		image(card) { return "../playing-cards/" + card.suit + "_" + card.card + ".png" },
		value(card) {
				if( !isNaN(Number(card)) ){ return Number(card) }

				if (card == 'A'){ return 14}
				if (card == 'K'){ return 13}
				if (card == 'Q'){ return 12}
				if (card == 'J'){ return 11}

				return 0;
		},

		// game methods
		draw(x) {
			const drawn = this.deck.slice(0, x);
			this.deck = this.deck.slice(x);
			return drawn;
		},
		estimate(card){
			const game = Alpine.store('game');
			const value = game.value(card.card);

			// only numeric cards have special purposes
			// also there are no diamonds/hearts with value above 10
			if(value <= 10){
				if(card.suit == 'diamonds'){
					// take up as weapon
					return `Take ${card.card} of ${card.suit} as weapon`;
				}

				if(card.suit == 'hearts'){
					// heal
					if(!this.healed){
						const new_hp = Math.min((this.hp + value) , 20);
						const hp_rec = new_hp - this.hp
						return `Recover ${hp_rec} hp`;
					} else {
						return "Already healed this turn,\nyou will heal for 0";
					}
				}
				// else combat
			}

			// battle with weapon
			if(this.weapon != null && this.equipped) {
				// has weapon and is equipped
				const top = this.durability.at(-1);
				const block = game.value(this.weapon.card)

				var dur = 15;
				if(top != null){ dur = game.value(top.card); }

				if(value < dur){
					// can block with weapon
					const lost_hp = Math.min( block - value , 0); // if value < block, take 0 damage rahter then healing

					const msg = `Will lose ${lost_hp} hp`
					return msg;
				} else {
					const lost_hp = value;
					const msg = `Can't block, too little durability: ${dur}<${value}\nWill lose ${lost_hp} hp`;
					return msg;
				}

			} 

			// bare-handed combat
			const lost_hp = value;
			const msg =  `Will lose ${lost_hp} hp`;
			return msg;
		},
		click_card(idx) {
			// remove element at idx
			const clicked = this.dungeon.splice(idx,1)[0];
			const game = Alpine.store('game');
			const value = game.value(clicked.card);

			// replenish cards
			if(this.dungeon.length == 1){
				this.dungeon.push(...game.draw(3));
				// rest once per turn effects
				this.healed = false;
				this.ran = false;
			}

			// only non-numeric cards have special purposes
			if(value <= 10){
				if(clicked.suit == 'diamonds'){
					// take up as weapon
					this.weapon = clicked;
					this.durability = []; // reset durability
					this.equipped = true; //default to equipping the weapon
					return;
				}

				if(clicked.suit == 'hearts'){
					// heal
					if(!this.healed){
						this.hp = Math.min((this.hp + value) , 20);
						this.healed = true;
					} else {
					}
					return;
				}
				// else combat
			}

			// battle
			if(this.weapon != null && this.equipped) {
				// has weapon and is equipped
				const top = this.durability.at(-1);
				const block = game.value(this.weapon.card)

				var dur = 15;
				if(top != null){ dur = game.value(top.card); }

				if(value < dur){
					// can block with weapon
					this.hp = this.hp + Math.min( block - value , 0); // if value < block, take 0 damage rahter then healing

					this.durability.push(clicked);
					if(this.durability.length > 4) this.durability.shift();
					return;
				} else {
				}

			} 

			// bare-handed combat
			this.hp = this.hp - value;
			return
		},
		reset(){
			this.deck = valid_cards.toSorted(() => Math.random() - 0.5);
			this.dungeon = Alpine.store('game').draw(4);
			this.weapon = null;
			this.durability = [];
			this.equipped = false;
			this.hp = 20;
			this.ran =  false;
			this.healed =  false;
		},
		run_away(){
			if(!this.ran){
				this.deck.push(...this.dungeon);
				this.dungeon = Alpine.store('game').draw(4);
				this.ran = true;
			} else {
			}
		}

});
});
