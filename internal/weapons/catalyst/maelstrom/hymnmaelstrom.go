package maelstrom

import (
	"fmt"

	"github.com/genshinsim/gcsim/pkg/core"
	"github.com/genshinsim/gcsim/pkg/core/attributes"
	"github.com/genshinsim/gcsim/pkg/core/event"
	"github.com/genshinsim/gcsim/pkg/core/glog"
	"github.com/genshinsim/gcsim/pkg/core/info"
	"github.com/genshinsim/gcsim/pkg/core/player/character"
	"github.com/genshinsim/gcsim/pkg/modifier"
)

const (
	StackingKey          = "maelstrom-stacking"
	stackingDuration     = 10 * 60
	reactionBuffKey      = "maelstrom-reaction-buff"
	reactionBuffDuration = 5 * 60
)

type Weapon struct {
	Index  int
	stacks int
}

func (w *Weapon) SetIndex(idx int) { w.Index = idx }
func (w *Weapon) Init() error      { return nil }

// Increases Healing Bonus by 4%.
//
//	When performing healing, the equipping character gains the "Vatsamonga's Vatic Vintage"
//
// effect, which increases Max HP by 4% as well as increases the currently active party
// member's ATK by 0.4% for every 1,000 Max HP the equipping character has over 40,000.
// A maximum of 8% ATK can be gained in this way. This effect lasts 10s, max 3 stacks.
// When a nearby party member triggers a Frozen or Stellar Swirl reaction, the aforementioned
// Max HP and ATK boosts will be further increased by 75% for the next 5s.
// The aforementioned effects can still trigger even when the equipping character is
// not on the field.
func NewWeapon(c *core.Core, char *character.CharWrapper, p info.WeaponProfile) (info.Weapon, error) {
	w := &Weapon{}
	multiplier := 0.075 + float64(p.Refine)*0.025

	mheal := make([]float64, attributes.EndStatType)
	mheal[attributes.Heal] = 0.04 * multiplier
	char.AddStatMod(character.StatMod{
		Base:         modifier.NewBase("maelstrom-heal-bonus", -1),
		AffectedStat: attributes.NoStat,
		Amount: func() []float64 {
			return mheal
		},
	})

	c.Events.Subscribe(event.OnHeal, func(args ...any) {
		source := args[0].(*info.HealInfo)
		amount := args[2].(float64)
		// check if the wearer realises the heal
		if source.Caller != char.Index() {
			return
		}
		if amount <= 0 {
			return
		}
		// reset stacks if status wasn't active
		if !char.StatusIsActive(StackingKey) {
			w.stacks = 0
		}
		// adds stack
		if w.stacks < 3 {
			w.stacks++
		}
		c.Log.NewEvent("maelstrom gains stack", glog.LogWeaponEvent, char.Index()).
			Write("stacks", w.stacks)
		char.AddStatus(StackingKey, stackingDuration, true)
	}, fmt.Sprintf("maelstrom-heal-%v", char.Base.Key.String()))

	// hp buff on stacks
	hp_boost := 0.04 * multiplier
	m := make([]float64, attributes.EndStatType)
	char.AddStatMod(character.StatMod{
		Base:         modifier.NewBaseWithHitlag("maelstrom-hp-buff", -1),
		AffectedStat: attributes.HP,
		Amount: func() []float64 {
			if !char.StatusIsActive(StackingKey) {
				return nil
			}
			boost := 1.0
			if char.StatusIsActive(reactionBuffKey) {
				boost = 1.75
			}
			m[attributes.HP] = hp_boost * float64(w.stacks) * boost
			return m
		},
	})

	// atk boost to active on stacks
	atk_boost := 0.004 * multiplier * float64(min(int((char.MaxHP()-40000)/1000), 2))
	active := c.Player.ActiveChar()
	active.AddStatMod(character.StatMod{
		Base:         modifier.NewBaseWithHitlag("maelstrom-atk-buff", -1),
		AffectedStat: attributes.ATKP,
		Amount: func() []float64 {
			if !char.StatusIsActive(StackingKey) {
				return nil
			}
			boost := 1.0
			if char.StatusIsActive(reactionBuffKey) {
				boost = 1.75
			}
			m[attributes.ATKP] = atk_boost * float64(w.stacks) * boost
			return m
		},
	})

	onBoostReaction := func(args ...any) {
		char.AddStatus(reactionBuffKey, reactionBuffDuration, true)
	}
	reactionSubKey := fmt.Sprintf("maelstrom-reaction-%v", char.Base.Key.String())
	c.Events.Subscribe(event.OnFrozen, onBoostReaction, reactionSubKey)
	c.Events.Subscribe(event.OnStellarSwirl, onBoostReaction, reactionSubKey)

	return w, nil
}
