package main

import (
	"math/rand"
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

type OpGameRepr struct {
	Deck_remaining int    `json:"deck"`
	Hp             int    `json:"hp"`
	Weapon         string `json:"weapon"`
	Played_card    string `json:"player_card"`
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

	new_cards := gast.deck[0:amount]
	gast.deck = gast.deck[amount:]

	return new_cards
}

func click_card(active_gast *Gamestate, idx int) (clicked Card, is_attack bool) {
	clicked = active_gast.dungeon[idx]
	// cards are an array of 4x13
	suit := Suit(clicked.value / 15)

	value := clicked.value

	// replenish cards
	if len(active_gast.dungeon) == 1 {
		drawn := draw(active_gast, 3)

		new_dungeon := append(active_gast.dungeon[:], drawn...)
		active_gast.dungeon = new_dungeon

		active_gast.healed = false
		active_gast.ran = false

	}

	// handle non-numeric cards
	if value <= 10 {
		switch suit {
		case Diamonds:
			{ // take weapon
				active_gast.weapon = clicked
				// 0 durability means no dur
				active_gast.durability = 0

				// greater then 0 weapon id means equipped
				return
			}
		case Hearts:
			{ // heal
				if !active_gast.healed {
					active_gast.hp = min(active_gast.hp+value, 20)
					active_gast.healed = true
				} else {
					// nothing happens
				}
				return

			}
		// else combat
		default:
		}
	}

	is_attack = true
	return
}

func attack(def_gast *Gamestate, attack Card) (blocked bool, defended bool) {
	motion_value := attack.value

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
	gast.hp = 20

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

// client payload
type Action struct {
	Toggle_weapon      bool `json:"toggle_eapon"`
	Ran                bool `json:"ran"`
	Clicked_card_index int  `json:"clicked_card_index"`
}

func Execute_action(active_gast *Gamestate, act Action) (selected Card, is_attack bool, failed bool) {
	if act.Toggle_weapon {
		active_gast.equipped = !active_gast.equipped
	}

	if act.Ran {
		failed = run_away(active_gast)
		if failed {
			return
		}
	}

	selected, is_attack = click_card(active_gast, act.Clicked_card_index)

	return
}
