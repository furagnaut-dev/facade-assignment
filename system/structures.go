package system

type User struct {
	ID int
	Username string
	Location string
	Rank string
}

type Lobby struct {
	LobbyID int
}

type Claim struct {
	ID int
	PlayerID int
	status string
}

func NewUser(id int, username, location, rank string) *User {
	return &User{
		ID: id,
		Username: username,
		Location: location,
		Rank: rank,
	}
}

func NewLobby(lobbyID int) *Lobby {
	return &Lobby{
		LobbyID: lobbyID,
	}
}

func NewClaim(claimID int, playerID int) *Claim {
	return &Claim{
		ID: claimID,
		PlayerID: playerID,
		status: "WAITING",
	}
}