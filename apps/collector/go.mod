module github.com/AchuthanDev/Network-Monitor-Debian/apps/collector

go 1.23

require github.com/AchuthanDev/Network-Monitor-Debian/features/network-usage v0.0.0

replace github.com/AchuthanDev/Network-Monitor-Debian/features/network-usage => ../../features/network-usage
