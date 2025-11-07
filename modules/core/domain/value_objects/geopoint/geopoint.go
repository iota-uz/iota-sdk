package geopoint

import "fmt"

type GeoPoint interface {
	Lat() float64
	Lng() float64
}

type geoPoint struct {
	lat float64
	lng float64
}

func (p geoPoint) Lat() float64 {
	return p.lat
}

func (p geoPoint) Lng() float64 {
	return p.lng
}

func (p geoPoint) String() string {
	return fmt.Sprintf("%f,%f", p.lat, p.lng)
}

func New(lat, lng float64) GeoPoint {
	return geoPoint{lat: lat, lng: lng}
}
