package gobritannia

import "strings"

// term maps a US English word to the preferred UK English word.
type term struct {
	american string
	british  string
}

// spellingTermEntries contains default spelling-only replacements.
var spellingTermEntries = []term{
	{american: "aerogram", british: "aerogramme"},
	{american: "aerograms", british: "aerogrammes"},
	{american: "aluminum", british: "aluminium"},
	{american: "analyze", british: "analyse"},
	{american: "analyzed", british: "analysed"},
	{american: "analyzer", british: "analyser"},
	{american: "analyzers", british: "analysers"},
	{american: "analyzes", british: "analyses"},
	{american: "analyzing", british: "analysing"},
	{american: "armor", british: "armour"},
	{american: "armored", british: "armoured"},
	{american: "armory", british: "armoury"},
	{american: "artifact", british: "artefact"},
	{american: "artifacts", british: "artefacts"},
	{american: "behavior", british: "behaviour"},
	{american: "behaviors", british: "behaviours"},
	{american: "canceled", british: "cancelled"},
	{american: "canceling", british: "cancelling"},
	{american: "catalog", british: "catalogue"},
	{american: "cataloged", british: "catalogued"},
	{american: "cataloging", british: "cataloguing"},
	{american: "catalogs", british: "catalogues"},
	{american: "center", british: "centre"},
	{american: "centered", british: "centred"},
	{american: "centering", british: "centring"},
	{american: "centers", british: "centres"},
	{american: "color", british: "colour"},
	{american: "colored", british: "coloured"},
	{american: "coloring", british: "colouring"},
	{american: "colors", british: "colours"},
	{american: "cozy", british: "cosy"},
	{american: "customize", british: "customise"},
	{american: "customized", british: "customised"},
	{american: "customizes", british: "customises"},
	{american: "customizing", british: "customising"},
	{american: "defense", british: "defence"},
	{american: "defenses", british: "defences"},
	{american: "dialog", british: "dialogue"},
	{american: "dialogs", british: "dialogues"},
	{american: "donut", british: "doughnut"},
	{american: "donuts", british: "doughnuts"},
	{american: "enroll", british: "enrol"},
	{american: "enrollment", british: "enrolment"},
	{american: "enrollments", british: "enrolments"},
	{american: "esthetic", british: "aesthetic"},
	{american: "flavor", british: "flavour"},
	{american: "flavored", british: "flavoured"},
	{american: "flavoring", british: "flavouring"},
	{american: "flavors", british: "flavours"},
	{american: "fulfill", british: "fulfil"},
	{american: "fulfillment", british: "fulfilment"},
	{american: "fulfills", british: "fulfils"},
	{american: "gray", british: "grey"},
	{american: "grays", british: "greys"},
	{american: "honor", british: "honour"},
	{american: "honored", british: "honoured"},
	{american: "honoring", british: "honouring"},
	{american: "honors", british: "honours"},
	{american: "initialize", british: "initialise"},
	{american: "initialized", british: "initialised"},
	{american: "initializes", british: "initialises"},
	{american: "initializing", british: "initialising"},
	{american: "jewelry", british: "jewellery"},
	{american: "kilometer", british: "kilometre"},
	{american: "kilometers", british: "kilometres"},
	{american: "labor", british: "labour"},
	{american: "labored", british: "laboured"},
	{american: "laborer", british: "labourer"},
	{american: "laborers", british: "labourers"},
	{american: "laboring", british: "labouring"},
	{american: "labors", british: "labours"},
	{american: "liter", british: "litre"},
	{american: "liters", british: "litres"},
	{american: "maneuver", british: "manoeuvre"},
	{american: "maneuvered", british: "manoeuvred"},
	{american: "maneuvering", british: "manoeuvring"},
	{american: "maneuvers", british: "manoeuvres"},
	{american: "meter", british: "metre"},
	{american: "meters", british: "metres"},
	{american: "mold", british: "mould"},
	{american: "molded", british: "moulded"},
	{american: "molding", british: "moulding"},
	{american: "molds", british: "moulds"},
	{american: "neighbor", british: "neighbour"},
	{american: "neighborhood", british: "neighbourhood"},
	{american: "neighborhoods", british: "neighbourhoods"},
	{american: "neighbors", british: "neighbours"},
	{american: "normalization", british: "normalisation"},
	{american: "normalize", british: "normalise"},
	{american: "normalized", british: "normalised"},
	{american: "normalizes", british: "normalises"},
	{american: "normalizing", british: "normalising"},
	{american: "odor", british: "odour"},
	{american: "odors", british: "odours"},
	{american: "offense", british: "offence"},
	{american: "offenses", british: "offences"},
	{american: "organize", british: "organise"},
	{american: "organized", british: "organised"},
	{american: "organizes", british: "organises"},
	{american: "organizing", british: "organising"},
	{american: "plow", british: "plough"},
	{american: "plowed", british: "ploughed"},
	{american: "plowing", british: "ploughing"},
	{american: "plows", british: "ploughs"},
	{american: "recognize", british: "recognise"},
	{american: "recognized", british: "recognised"},
	{american: "recognizes", british: "recognises"},
	{american: "recognizing", british: "recognising"},
	{american: "savory", british: "savoury"},
	{american: "skeptic", british: "sceptic"},
	{american: "skeptical", british: "sceptical"},
	{american: "skeptics", british: "sceptics"},
	{american: "theater", british: "theatre"},
	{american: "theaters", british: "theatres"},
	{american: "traveled", british: "travelled"},
	{american: "traveler", british: "traveller"},
	{american: "travelers", british: "travellers"},
	{american: "traveling", british: "travelling"},
	{american: "tumor", british: "tumour"},
	{american: "tumors", british: "tumours"},
	{american: "yogurt", british: "yoghurt"},
}

// vocabularyTermEntries contains default vocabulary replacements.
var vocabularyTermEntries = []term{
	{american: "airplane", british: "aeroplane"},
	{american: "airplanes", british: "aeroplanes"},
	{american: "bangs", british: "fringe"},
	{american: "cellphone", british: "mobile"},
	{american: "cellphones", british: "mobiles"},
	{american: "cookie", british: "biscuit"},
	{american: "cookies", british: "biscuits"},
	{american: "freeway", british: "motorway"},
	{american: "freeways", british: "motorways"},
	{american: "garbage", british: "rubbish"},
	{american: "gasoline", british: "petrol"},
	{american: "hood", british: "bonnet"},
	{american: "hoods", british: "bonnets"},
	{american: "ladybug", british: "ladybird"},
	{american: "ladybugs", british: "ladybirds"},
	{american: "movie", british: "film"},
	{american: "movies", british: "films"},
	{american: "pacifier", british: "dummy"},
	{american: "pacifiers", british: "dummies"},
	{american: "sidewalk", british: "pavement"},
	{american: "sidewalks", british: "pavements"},
	{american: "sneaker", british: "trainer"},
	{american: "sneakers", british: "trainers"},
	{american: "trunk", british: "boot"},
	{american: "trunks", british: "boots"},
}

// allowedTerms contains separate allowlists for code identifiers and comments.
type allowedTerms struct {
	code    map[string]bool
	comment map[string]bool
}

// newBritishTerms builds the active replacement lookup.
func newBritishTerms() map[string]string {
	entries := append([]term{}, spellingTermEntries...)
	entries = append(entries, vocabularyTermEntries...)

	terms := make(map[string]string, len(entries))
	for _, entry := range entries {
		terms[entry.american] = entry.british
	}

	return terms
}

// newAllowedTerms builds lower-case lookups for terms that should not be reported.
func newAllowedTerms(entries []AllowTerm) allowedTerms {
	terms := allowedTerms{
		code:    make(map[string]bool, len(entries)),
		comment: make(map[string]bool, len(entries)),
	}
	for _, entry := range entries {
		word := normaliseTerm(entry.Term)
		if word == "" {
			continue
		}

		terms.code[word] = true
		if allowComments(entry) {
			terms.comment[word] = true
		}
	}

	return terms
}

// normaliseTerm returns the canonical lower-case form for an allowlist term.
func normaliseTerm(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
