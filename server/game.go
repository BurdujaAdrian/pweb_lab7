package main

import (
	"fmt"
	"math/rand"
	"slices"
	"sync"
)

// game parameters
const (
	MAX_HP = 40
)

type Suit uint

const (
	Clubs Suit = iota
	Diamonds
	Hearts
	Spades
	Suits_count
)

type Card struct {
	value int
	suit  Suit
}

// each player has one
type Gamestate struct {
	deck     []Card
	equipped bool
	weapon   Card
	// frondend keeps track of the Cards in durability
	durability int
	ran        bool
	healed     bool
	dungeon    []Card
	hp         int
}
type ActionResult struct {
	Failed   bool `json:"failed"`
	Blocked  bool `json:"blocked"`
	Defended bool `json:"defended"`
}
type GameAction struct {
	selected  Card
	is_attack bool
	failed    bool
}
type Player struct {
	Mutex     *sync.Mutex `json:"-"`
	Name      string
	gamestate *Gamestate
	comm      chan Card
}

type OpGameRepr struct {
	Deck_remaining int    `json:"deck"`
	Hp             int    `json:"hp"`
	Weapon         string `json:"weapon"`
	Played_card    string `json:"player_card"`
}

func format_op_gamestate(gamestate *Gamestate, card Card) OpGameRepr {
	return OpGameRepr{
		Deck_remaining: len(gamestate.deck),
		Hp:             gamestate.hp,
		Weapon:         card_to_repr(gamestate.weapon),
		Played_card:    card_to_repr(card),
	}
}

type GameRepr struct {
	Deck_remaining int `json:"deck"`
	// no string=no weapon
	Weapon   string `json:"weapon"`
	Equipped bool   `json:"equipped"`
	// front end keeps track of it's own cards in Durability
	Durability int      `json:"durability"`
	Ran        bool     `json:"ran"`
	Healed     bool     `json:"healed"`
	Dungeon    []string `json:"dungeon"`
	Hp         int      `json:"hp"`
}

// result.New_gamestate = format_gamestate(player.gamestate)
func format_gamestate(gast *Gamestate) (new_repr GameRepr) {
	new_repr.Deck_remaining = len(gast.deck)

	var dungeon_cards []string
	for _, card := range gast.dungeon {
		dungeon_cards = append(dungeon_cards, card_to_repr(card))
	}
	new_repr.Dungeon = dungeon_cards

	new_repr.Durability = gast.durability

	new_repr.Healed = gast.healed

	new_repr.Hp = gast.hp

	new_repr.Ran = gast.ran

	new_repr.Equipped = gast.equipped

	new_repr.Weapon = card_to_repr(gast.weapon)

	return
}
func index_to_card(idx int) Card  { return Card{value: idx%13 + 2, suit: Suit(idx / 13)} }
func card_to_index(card Card) int { return card.value - 2 + int(card.suit)*13 }

func card_to_repr(card Card) string {
	index := card_to_index(card)
	if index < 0 {
		return ""
	}
	return Cards[index]
}

var Cards [4 * 13]string

func Populate_cards_index() {
	suits := []string{"clubs", "diamonds", "hearts", "spades"}
	cards := []string{"2", "3", "4", "5", "6", "7", "8", "9", "10", "J", "Q", "K", "A"}

	for i, suit := range suits {
		for j, card := range cards {
			Cards[i*13+j] = suit + card
		}
	}
}

var default_deck []Card

func Populate_default_deck() {
	for i := range Cards {
		card := index_to_card(i)
		if card.value > 10 {
			if card.suit == Diamonds || card.suit == Hearts {
				continue
			}
		}

		default_deck = append(default_deck, card)
	}

	if len(default_deck) != 44 {
		panic("populated default deck incorrectly")
	}
}

func draw(gast *Gamestate, amount int) []Card {
	if amount < 1 {
		panic("asked to draw less then 1 card")
	}

	if amount >= len(gast.deck) {
		amount = len(gast.deck)
	}
	new_cards := gast.deck[:amount]
	gast.deck = gast.deck[amount:]

	return new_cards
}

func click_card(active_gast *Gamestate, idx int) (clicked Card, failed bool) {
	if idx >= len(active_gast.dungeon) {
		failed = true
		return
	}
	clicked = active_gast.dungeon[idx]
	active_gast.dungeon = slices.Delete(active_gast.dungeon, idx, idx+1)
	// cards are an array of 4x13
	suit := clicked.suit

	value := clicked.value

	// handle non-numeric cards
	if value <= 10 {
		switch suit {
		case Diamonds:
			{ // take weapon
				active_gast.weapon = clicked
				// 0 durability means no dur
				active_gast.durability = 0

				// greater then 0 weapon id means equipped
			}
		case Hearts:
			{ // heal
				if !active_gast.healed {
					active_gast.hp = min(active_gast.hp+value, MAX_HP)
					active_gast.healed = true
				}
			}
		}
	}

	// replenish cards
	if len(active_gast.dungeon) == 1 {
		drawn := draw(active_gast, 3)

		new_dungeon := append(active_gast.dungeon[:], drawn...)
		active_gast.dungeon = new_dungeon

		active_gast.healed = false
		active_gast.ran = false
	}

	return
}

func resolve_attack(def_gast *Gamestate, attack Card) (blocked bool, defended bool) {
	motion_value := attack.value
	// no attack was sent
	if motion_value == 0 {
		return
	}

	switch attack.suit {
	case Diamonds, Hearts:
		return
	}

	// if a weapon is pressent and equipped
	if def_gast.equipped && def_gast.weapon.value > 1 {
		defended = true
		dur := def_gast.durability
		weapon := def_gast.weapon
		block := weapon.value

		if dur == 0 {
			dur = 15
		}

		// can block with weapon
		if motion_value < dur {
			blocked = true

			def_gast.hp = def_gast.hp + min(block-motion_value, 0)

			def_gast.durability = motion_value

			return
		} else {
			// failed to block
		}
	}

	// barehanded combat
	def_gast.hp -= motion_value

	return
}

func reset(gast *Gamestate) {
	new_deck := append([]Card{}, default_deck...)
	rand.Shuffle(44, func(i, j int) {
		new_deck[i], new_deck[j] = new_deck[j], new_deck[i]
	})

	*gast = Gamestate{}
	gast.deck = new_deck
	gast.hp = MAX_HP

	gast.dungeon = draw(gast, 4)
}

func run_away(active_gast *Gamestate) (denied bool) {
	if !active_gast.ran {
		active_gast.deck = append(active_gast.deck, active_gast.dungeon[:]...)
		active_gast.ran = true

		new_dungeon := draw(active_gast, 4)
		active_gast.dungeon = new_dungeon
	} else {
		denied = true
	}
	return
}

type WinState int

const (
	GAME_ON WinState = iota
	YOU_WIN
	YOU_LOSE
	TIE
)

func (w WinState) String() string {
	switch w {
	case GAME_ON:
		return "GAME_ON"
	case YOU_WIN:
		return "YOU_WIN"
	case YOU_LOSE:
		return "YOU_LOSE"
	case TIE:
		return "TIE"
	default:
		return fmt.Sprintf("WinState(%d)", w)
	}
}

func Check_win(active_gast *Gamestate, op_gast *Gamestate) WinState {
	// check hp
	if active_gast.hp <= 0 {
		// if both below 0, it's a tie
		if op_gast.hp <= 0 {
			return TIE
		}

		return YOU_LOSE
	}

	if op_gast.hp <= 0 {
		return YOU_WIN
	}

	// else, check remaining cards
	// INFO: the only time dungeon can be empty is when all cards have been played
	// also, both player's deck always have the same size
	if len(active_gast.dungeon) == 0 {
		if active_gast.hp == op_gast.hp {
			return TIE
		}

		if active_gast.hp > op_gast.hp {
			return YOU_WIN
		} else {
			return YOU_LOSE
		}
	}

	// else, the game continues
	return GAME_ON
}

// client payload
type Action struct {
	Toggle_weapon      bool `json:"toggle_eapon"`
	Ran                bool `json:"ran"`
	Clicked_card_index int  `json:"clicked_card_index"`
}

func Execute_action(active_gast *Gamestate, act Action) (selected Card, failed bool) {
	if act.Toggle_weapon {
		active_gast.equipped = !active_gast.equipped
	}

	selected, failed = click_card(active_gast, act.Clicked_card_index)

	return
}
