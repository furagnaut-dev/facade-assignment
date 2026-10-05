package system

import "fmt"

func (c *Claim) LobbyFormation(id int) error {
	if c.status == "WAITING" {
		lobby := NewLobby(id)
		if lobby.LobbyID <= 0 {
			return fmt.Errorf("No lobbies found")
		} else {
			c.status = "HELD"
		}
	}
	return nil
}