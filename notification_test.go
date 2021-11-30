package lime

func createNotification() *Notification {
	n := Notification{}
	n.ID = "4609d0a3-00eb-4e16-9d44-27d115c6eb31"
	n.To = Node{}
	n.To.Name = "golang"
	n.To.Domain = "limeprotocol.org"
	n.To.Instance = "default"
	n.Event = NotificationEventReceived
	return &n
}
