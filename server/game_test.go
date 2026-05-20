package main

import (
	"fmt"
	"maps"
	"os"
	"slices"
	"testing"
)

func TestMain(m *testing.M) {

	Populate_cards_index()
	Populate_default_deck()

	code := m.Run()

	os.Exit(code)
}
func Test_test(t *testing.T) {
	fmt.Print("")
}

func Test_card_conversions(t *testing.T) {
	for idx := range 12 {
		for suit := range Suits_count {
			card := Card{
				value: idx + 2,
				suit:  suit,
			}

			card_index := card_to_index(card)
			index_card := index_to_card(card_index)
			index_card_index := card_to_index(index_card)

			if card != index_card {
				t.Errorf("Idx:%v suit:%v Card != to index_card: %v != %v", idx, suit, card, index_card)
			}

			if card_index != index_card_index {
				t.Errorf("Idx:%v suit:%v Card_index != to index_card_index: %v != %v", idx, suit, card_index, index_card_index)

			}

		}
	}
}

func TestDraw(t *testing.T) {
	test_deck := []Card{
		{},
		{},
		{},
		{},
	}
	draw_gs := Gamestate{
		deck: test_deck,
	}

	cards_drawn := draw(&draw_gs, 4)

	if len(cards_drawn) != 4 {
		t.Errorf("draw was supposed to draw 4 cards, drew %v instead", len(cards_drawn))
	}

	the_same := slices.Equal(test_deck, cards_drawn)
	if !the_same {
		t.Errorf("the cards drawn are not the same as the cards in deck: %v != \n %v", cards_drawn, test_deck)
	}

	draw_gs.deck = default_deck
	cards_drawn = draw(&draw_gs, 4)
	expected_drawn := default_deck[:4]

	if len(cards_drawn) != len(expected_drawn) {
		t.Errorf("Expected the slices to have the same size, got %v %v", cards_drawn, expected_drawn)
	}

	the_same = slices.Equal(expected_drawn, cards_drawn)
	if !the_same {
		t.Errorf("Expected the slices to be the same, got %v %v", cards_drawn, expected_drawn)
	}

	draw_gs.deck = []Card{{}, {}}
	cards_drawn = draw(&draw_gs, 4)
	expected_drawn = []Card{{}, {}}
	if len(cards_drawn) != len(expected_drawn) {
		t.Errorf("Expected the slices to have the same size, got %v %v", cards_drawn, expected_drawn)
	}

	the_same = slices.Equal(expected_drawn, cards_drawn)
	if !the_same {
		t.Errorf("Expected the slices to be the same, got %v %v", cards_drawn, expected_drawn)
	}
}

func TestClick(t *testing.T) {

	if len(default_deck) == 0 {
		Populate_default_deck()
	}

	newTestGamestate := func(dungeon []Card) *Gamestate {
		gs := &Gamestate{
			hp:      MAX_HP,
			deck:    append([]Card{}, default_deck...),
			dungeon: append([]Card{}, dungeon...),
		}
		return gs
	}

	t.Run("click diamond equips weapon, no attack, no heal", func(t *testing.T) {
		diamondCard := Card{value: 5, suit: Diamonds}
		gs := newTestGamestate([]Card{
			{value: 3, suit: Clubs},
			diamondCard,
			{value: 7, suit: Spades},
			{value: 2, suit: Hearts},
		})

		clicked, valid := click_card(gs, 1)

		if !valid {
			t.Fatalf("expected valid click, got invalid; clicked: %v", clicked)
		}
		if clicked != diamondCard {
			t.Errorf("clicked card = %+v, want %+v", clicked, diamondCard)
		}

		if gs.weapon != diamondCard {
			t.Errorf("weapon not equipped: got %+v", gs.weapon)
		}

		if gs.durability != 0 {
			t.Errorf("durability should be 0 after equipping, got %d", gs.durability)
		}

		if gs.hp != MAX_HP {
			t.Errorf("hp should not change when equipping a weapon, got %d", gs.hp)
		}

		if len(gs.dungeon) != 3 {
			t.Errorf("dungeon length should be 3 after click, got %d", len(gs.dungeon))
		}

		if gs.healed {
			t.Error("healed should remain false after diamond")
		}
	})

	t.Run("click heart heals, no attack", func(t *testing.T) {
		heartCard := Card{value: 4, suit: Hearts}
		gs := newTestGamestate([]Card{
			{value: 3, suit: Clubs},
			heartCard,
			{value: 7, suit: Spades},
			{value: 9, suit: Clubs},
		})

		clicked, valid := click_card(gs, 1)

		if !valid {
			t.Fatal("expected valid click")
		}
		if clicked != heartCard {
			t.Errorf("clicked card = %+v, want %+v", clicked, heartCard)
		}

		if gs.hp != MAX_HP {
			t.Errorf("hp should be capped at MAX_HP, got %d", gs.hp)
		}

		if !gs.healed {
			t.Error("healed should be true after first heart")
		}

		var emptyCard Card
		if gs.weapon != emptyCard {
			t.Error("weapon should remain empty after heart")
		}
	})

	t.Run("click heart when already healed in same room does nothing", func(t *testing.T) {
		firstHeart := Card{value: 3, suit: Hearts}
		secondHeart := Card{value: 5, suit: Hearts}
		gs := newTestGamestate([]Card{
			firstHeart,
			{value: 10, suit: Clubs},
			secondHeart,
			{value: 2, suit: Spades},
		})

		click_card(gs, 0)
		hpBefore := gs.hp

		clicked, valid := click_card(gs, 1)

		if !valid {
			t.Fatal("expected valid click")
		}
		if clicked != secondHeart {
			t.Errorf("clicked card = %+v, want %+v", clicked, secondHeart)
		}

		if gs.hp != hpBefore {
			t.Errorf("hp should not change on second heart, got %d", gs.hp)
		}

		if !gs.healed {
			t.Error("healed should remain true")
		}
	})

	t.Run("room refill resets healed and ran when dungeon becomes 1", func(t *testing.T) {

		gs := newTestGamestate([]Card{
			{value: 2, suit: Hearts},
			{value: 6, suit: Diamonds},
		})

		gs.healed = true
		gs.ran = true
		click_card(gs, 0)

		if len(gs.dungeon) != 4 {
			t.Fatalf("after refill dungeon length should be 4, got %d", len(gs.dungeon))
		}

		if gs.healed {
			t.Errorf("healed should be reset after room refill, dungeon:%#v", gs)
		}
		if gs.ran {
			t.Error("ran should be reset after room refill")
		}
	})

	t.Run("heal capping at MAX_HP", func(t *testing.T) {

		gs := newTestGamestate([]Card{
			{value: 8, suit: Hearts},
			{value: 3, suit: Clubs},
			{value: 4, suit: Spades},
			{value: 9, suit: Diamonds},
		})
		gs.hp = MAX_HP - 8 + 1

		click_card(gs, 0)
		if gs.hp != MAX_HP {
			t.Errorf("hp should be capped at MAX_HP, got %d", gs.hp)
		}
	})

	t.Run("invalid index returns valid=false", func(t *testing.T) {
		gs := newTestGamestate([]Card{
			{value: 2, suit: Hearts},
			{value: 3, suit: Clubs},
		})
		_, valid := click_card(gs, 5)
		if valid {
			t.Error("expected invalid for out-of-bounds index")
		}
	})
}

func TestResolveAttack(t *testing.T) {

	newDefender := func(equipped bool, weapon Card, durability int, hp int) *Gamestate {
		return &Gamestate{
			equipped:   equipped,
			weapon:     weapon,
			durability: durability,
			hp:         hp,
		}
	}

	t.Run("barehanded – no weapon", func(t *testing.T) {
		def := newDefender(false, Card{}, 0, MAX_HP)
		expected_damage := MAX_HP - 6
		blocked, defended := resolve_attack(def, Card{value: 6, suit: Clubs})
		if defended {
			t.Error("defended should be false without weapon")
		}
		if blocked {
			t.Error("blocked should be false without weapon")
		}
		if def.hp != expected_damage {
			t.Errorf("full damage not taken: hp=%d, want %v", def.hp, expected_damage)
		}
	})

	t.Run("barehanded – weapon equipped but value ≤ 1", func(t *testing.T) {
		def := newDefender(true, Card{value: 1, suit: Diamonds}, 0, MAX_HP)
		_, _ = resolve_attack(def, Card{value: 5, suit: Spades})
		if def.hp != MAX_HP-5 {
			t.Errorf("should take full damage: hp=%d, want %v", def.hp, MAX_HP-5)
		}
	})

	t.Run("fresh weapon (dur=0) blocks attack, no damage", func(t *testing.T) {
		def := newDefender(true, Card{value: 10, suit: Diamonds}, 0, MAX_HP)
		attack := Card{value: 8, suit: Clubs}
		blocked, defended := resolve_attack(def, attack)

		if !defended {
			t.Error("defended should be true")
		}
		if !blocked {
			t.Error("blocked should be true – attack < dur (15)")
		}

		if def.durability != 8 {
			t.Errorf("durability should be %d, got %d", 8, def.durability)
		}

		if def.hp != MAX_HP {
			t.Errorf("hp should be MAX_HP, got %d", def.hp)
		}
	})

	t.Run("weapon blocks weaker attack, no damage", func(t *testing.T) {

		def := newDefender(true, Card{value: 10, suit: Diamonds}, 10, MAX_HP)
		attack := Card{value: 7, suit: Spades}
		blocked, _ := resolve_attack(def, attack)

		if !blocked {
			t.Error("attack < dur should be blocked")
		}
		if def.durability != 7 {
			t.Errorf("durability should become 7, got %d", def.durability)
		}
		if def.hp != MAX_HP {
			t.Errorf("no damage expected, hp=%d", def.hp)
		}
	})

	t.Run("weapon blocks attack, weapon value greater than attack", func(t *testing.T) {

		def := newDefender(true, Card{value: 12, suit: Diamonds}, 10, MAX_HP)
		attack := Card{value: 9, suit: Clubs}
		_, _ = resolve_attack(def, attack)
		if def.hp != MAX_HP {
			t.Errorf("hp should be MAX_HP (full block), got %d", def.hp)
		}
		if def.durability != 9 {
			t.Errorf("durability should be 9, got %d", def.durability)
		}
	})

	t.Run("weapon partially blocks damage", func(t *testing.T) {

		def := newDefender(true, Card{value: 5, suit: Diamonds}, 8, MAX_HP)
		attack := Card{value: 8, suit: Clubs}
		blocked, _ := resolve_attack(def, attack)

		if blocked {
			t.Error("attack not < durability → should not block")
		}
		if def.hp != MAX_HP-8 {
			t.Errorf("full partial expected (3), hp=%d", def.hp)
		}
		if def.durability != 8 {
			t.Errorf("durability unchanged after failed block, got %d", def.durability)
		}

		def2 := newDefender(true, Card{value: 5, suit: Diamonds}, 10, MAX_HP)
		blocked2, _ := resolve_attack(def2, Card{value: 8, suit: Clubs})
		if !blocked2 {
			t.Error("should block because 8 < 10")
		}
		if def2.hp != MAX_HP-3 {
			t.Errorf("partial block damage 3, hp=%d", def2.hp)
		}
		if def2.durability != 8 {
			t.Errorf("durability should become 8, got %d", def2.durability)
		}
	})

	t.Run("weapon fails because attack >= durability, full damage", func(t *testing.T) {
		def := newDefender(true, Card{value: 9, suit: Diamonds}, 5, MAX_HP)
		attack := Card{value: 6, suit: Clubs}
		blocked, defended := resolve_attack(def, attack)
		if !defended {
			t.Error("defended should be true because weapon equipped")
		}
		if blocked {
			t.Error("blocked should be false")
		}
		if def.hp != MAX_HP-6 {
			t.Errorf("full damage 6, hp=%d", def.hp)
		}
		if def.durability != 5 {
			t.Errorf("durability unchanged, got %d", def.durability)
		}
	})

	t.Run("weapon durability 0 sets dur=15, blocks up to 14", func(t *testing.T) {
		def := newDefender(true, Card{value: 6, suit: Diamonds}, 0, MAX_HP)
		attack := Card{value: 14, suit: Spades}
		blocked, _ := resolve_attack(def, attack)
		if !blocked {
			t.Error("should block because 14 < 15")
		}

		if def.hp != MAX_HP-8 {
			t.Errorf("hp should be 12, got %d", def.hp)
		}
		if def.durability != 14 {
			t.Errorf("durability should become 14, got %d", def.durability)
		}
	})

	t.Run("non‑attack heart does nothing to defender", func(t *testing.T) {
		def := newDefender(true, Card{value: 8, suit: Diamonds}, 12, 15)
		heart := Card{value: 3, suit: Hearts}
		blocked, defended := resolve_attack(def, heart)
		if blocked || defended {
			t.Error("no block/defend for non‑attack")
		}
		if def.hp != 15 {
			t.Error("hp should not change")
		}
		if def.durability != 12 {
			t.Error("durability should not change")
		}
	})

	t.Run("non‑attack diamond does nothing to defender", func(t *testing.T) {
		def := newDefender(true, Card{value: 7, suit: Diamonds}, 6, 18)
		diamond := Card{value: 9, suit: Diamonds}
		blocked, defended := resolve_attack(def, diamond)
		if blocked || defended {
			t.Error("no combat flags for opponent equipping weapon")
		}
		if def.hp != 18 || def.durability != 6 {
			t.Error("state unchanged")
		}
	})

	t.Run("non‑attack card with high value still harmless", func(t *testing.T) {
		def := newDefender(false, Card{}, 0, 10)
		heart := Card{value: 14, suit: Hearts}
		_, _ = resolve_attack(def, heart)
		if def.hp != 10 {
			t.Error("high heart should not damage")
		}
	})
}

func TestReset(t *testing.T) {

	if len(default_deck) == 0 {
		Populate_default_deck()
	}

	gs := &Gamestate{
		deck:       []Card{{value: 1, suit: Spades}},
		equipped:   true,
		weapon:     Card{value: 5, suit: Diamonds},
		durability: 3,
		ran:        true,
		healed:     true,
		dungeon:    []Card{{value: 10, suit: Hearts}},
		hp:         5,
	}

	reset(gs)

	if gs.hp != MAX_HP {
		t.Errorf("hp should be %d, got %d", MAX_HP, gs.hp)
	}
	if gs.equipped {
		t.Error("equipped should be false")
	}
	var empty Card
	if gs.weapon != empty {
		t.Errorf("weapon should be zero value, got %+v", gs.weapon)
	}
	if gs.durability != 0 {
		t.Errorf("durability should be 0, got %d", gs.durability)
	}
	if gs.ran {
		t.Error("ran should be false")
	}
	if gs.healed {
		t.Error("healed should be false")
	}

	if len(gs.dungeon) != 4 {
		t.Fatalf("dungeon length should be 4, got %d", len(gs.dungeon))
	}
	expectedDeckLen := len(default_deck) - 4
	if len(gs.deck) != expectedDeckLen {
		t.Errorf("deck length should be %d, got %d", expectedDeckLen, len(gs.deck))
	}

	count := func(cards []Card) map[Card]int {
		m := make(map[Card]int)
		for _, c := range cards {
			m[c]++
		}
		return m
	}
	combined := append(append([]Card{}, gs.deck...), gs.dungeon...)
	if !maps.Equal(count(combined), count(default_deck)) {
		t.Error("deck + dungeon does not contain the same multiset as default_deck – cards missing or duplicated")
	}

	if slices.Equal(gs.deck, default_deck[:40]) {
		t.Error("deck appears unshuffled – it matches the original default_deck order (extremely unlikely unless shuffle didn't happen)")
	}

	if slices.Equal(gs.dungeon, default_deck[40:]) {
		t.Error("dungeon matches the original last 4 cards of default_deck – shuffle may not have run")
	}
}

func TestRunAway(t *testing.T) {

	if len(default_deck) == 0 {
		Populate_default_deck()
	}

	cardCount := func(cards []Card) map[Card]int {
		m := make(map[Card]int)
		for _, c := range cards {
			m[c]++
		}
		return m
	}

	t.Run("successful run away", func(t *testing.T) {

		gs := &Gamestate{}
		customDeck := []Card{
			{value: 2, suit: Clubs},
			{value: 3, suit: Clubs},
			{value: 4, suit: Clubs},
			{value: 5, suit: Clubs},
			{value: 6, suit: Clubs},
			{value: 7, suit: Clubs},
			{value: 8, suit: Clubs},
			{value: 9, suit: Clubs},
			{value: 10, suit: Clubs},
			{value: 2, suit: Spades},
		}
		initialDungeon := []Card{
			{value: 2, suit: Hearts},
			{value: 3, suit: Hearts},
			{value: 4, suit: Hearts},
			{value: 5, suit: Hearts},
		}
		gs = &Gamestate{
			deck:    customDeck,
			dungeon: initialDungeon,
			ran:     false,
		}

		oldDeckLen := len(gs.deck)
		oldDungeon := append([]Card{}, gs.dungeon...)

		beforeAll := append(append([]Card{}, gs.deck...), gs.dungeon...)
		beforeCount := cardCount(beforeAll)

		denied := run_away(gs)

		if denied {
			t.Error("run_away should not be denied on first attempt")
		}
		if !gs.ran {
			t.Error("ran flag should be true after run_away")
		}

		expectedDeckLen := oldDeckLen
		if len(gs.deck) != expectedDeckLen {
			t.Errorf("deck length unchanged, expected %d, got %d", expectedDeckLen, len(gs.deck))
		}

		if len(gs.dungeon) != 4 {
			t.Errorf("dungeon length should be 4 after run, got %d", len(gs.dungeon))
		}

		if slices.Equal(gs.dungeon, oldDungeon) {
			t.Error("dungeon should be new cards, not the same as old dungeon")
		}

		afterAll := append(append([]Card{}, gs.deck...), gs.dungeon...)
		afterCount := cardCount(afterAll)
		if !maps.Equal(beforeCount, afterCount) {
			t.Error("total card multiset changed after run_away")
		}

	})

	t.Run("run away when already ran", func(t *testing.T) {
		gs := &Gamestate{
			deck:    []Card{{value: 2, suit: Clubs}, {value: 3, suit: Clubs}, {value: 4, suit: Clubs}, {value: 5, suit: Clubs}, {value: 6, suit: Clubs}},
			dungeon: []Card{{value: 7, suit: Hearts}, {value: 8, suit: Hearts}, {value: 9, suit: Hearts}, {value: 10, suit: Hearts}},
			ran:     true,
		}
		deckBefore := append([]Card{}, gs.deck...)
		dungeonBefore := append([]Card{}, gs.dungeon...)

		denied := run_away(gs)

		if !denied {
			t.Error("run_away should be denied when ran flag is true")
		}
		if !gs.ran {
			t.Error("ran flag should remain true")
		}
		if !slices.Equal(gs.deck, deckBefore) {
			t.Error("deck changed after denied run_away")
		}
		if !slices.Equal(gs.dungeon, dungeonBefore) {
			t.Error("dungeon changed after denied run_away")
		}
	})

	t.Run("run away with empty dungeon (edge case)", func(t *testing.T) {

		gs := &Gamestate{
			deck:    []Card{{value: 2, suit: Clubs}, {value: 3, suit: Clubs}, {value: 4, suit: Clubs}, {value: 5, suit: Clubs}, {value: 6, suit: Clubs}},
			dungeon: []Card{},
			ran:     false,
		}
		denied := run_away(gs)
		if denied {
			t.Error("run_away should succeed even with empty dungeon")
		}
		if len(gs.dungeon) != 4 {
			t.Fatalf("expected dungeon of 4 after run, got %d", len(gs.dungeon))
		}
		if len(gs.deck) != 1 {
			t.Errorf("expected deck length 1, got %d", len(gs.deck))
		}

	})
}
