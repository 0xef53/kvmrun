package osprober

func VerByCode(m map[string]string, code string) string {
	for k, v := range m {
		if v == code {
			return k
		}
	}
	return ""
}

var DEBIAN_CODES = map[string]string{
	"4":  "Etch",
	"5":  "Lenny",
	"6":  "Squeeze",
	"7":  "Wheezy",
	"8":  "Jessie",
	"9":  "Stretch",
	"10": "Buster",
}

var UBUNTU_CODES = map[string]string{
	"4.10":  "Warty Warthog",
	"5.04":  "Hoary Hedgehog",
	"5.10":  "Breezy Badger",
	"6.06":  "Dapper Drake",
	"6.10":  "Edgy Eft",
	"7.04":  "Feisty Fawn",
	"7.10":  "Gutsy Gibbon",
	"8.04":  "Hardy Heron",
	"8.10":  "Intrepid Ibex",
	"9.04":  "Jaunty Jackalope",
	"9.10":  "Karmic Koala",
	"10.04": "Lucid Lynx",
	"10.10": "Maverick Meerkat",
	"11.04": "Natty Narwhal",
	"11.10": "Oneiric Ocelot",
	"12.04": "Precise Pangolin",
	"12.10": "Quantal Quetzal",
	"13.04": "Raring Ringtail",
	"13.10": "Saucy Salamander",
	"14.04": "Trusty Tahr",
	"14.10": "Utopic Unicorn",
	"15.04": "Vivid Vervet",
	"15.10": "Wily Werewolf",
	"16.04": "Xenial Xerus",
	"16.10": "Yakkety Yak",
	"17.04": "Zesty Zapus",
	"17.10": "Artful Aardvark",
	"18.04": "Bionic Beaver",
	"18.10": "Cosmic Cuttlefish",
	"19.04": "Disco Dingo",
	"19.10": "Eoan Ermine",
	"20.04": "Focal Fossa",
	"20.10": "Groovy Gorilla",
}
