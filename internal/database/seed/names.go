// Package seed provides data generation for populating a vault.
package seed

// Surnames is a curated list of surnames for generating residents.
// Mix of common American surnames from various backgrounds.
var Surnames = []string{
	"Adams", "Anderson", "Baker", "Barnes", "Bell", "Bennett", "Brooks",
	"Brown", "Butler", "Campbell", "Carter", "Chen", "Clark", "Collins",
	"Cooper", "Cruz", "Davis", "Diaz", "Edwards", "Evans", "Fisher",
	"Flores", "Foster", "Garcia", "Gonzalez", "Gray", "Green", "Hall",
	"Harris", "Hayes", "Henderson", "Hernandez", "Hill", "Howard", "Hughes",
	"Jackson", "James", "Jenkins", "Johnson", "Jones", "Kelly", "Kim",
	"King", "Lee", "Lewis", "Long", "Lopez", "Martin", "Martinez",
	"Miller", "Mitchell", "Moore", "Morgan", "Morris", "Murphy", "Nelson",
	"Nguyen", "Parker", "Patterson", "Perez", "Perry", "Peterson", "Phillips",
	"Powell", "Price", "Ramirez", "Reed", "Reyes", "Richardson", "Rivera",
	"Roberts", "Robinson", "Rodriguez", "Rogers", "Ross", "Russell", "Sanchez",
	"Sanders", "Scott", "Simmons", "Smith", "Stewart", "Sullivan", "Taylor",
	"Thomas", "Thompson", "Torres", "Turner", "Walker", "Ward", "Washington",
	"Watson", "White", "Williams", "Wilson", "Wood", "Wright", "Young",
}

// MaleGivenNames is a curated list of male first names.
var MaleGivenNames = []string{
	"Aaron", "Adam", "Adrian", "Alan", "Albert", "Alexander", "Andrew",
	"Anthony", "Arthur", "Benjamin", "Brandon", "Brian", "Bruce", "Carl",
	"Charles", "Christopher", "Daniel", "David", "Dennis", "Donald", "Douglas",
	"Edward", "Eric", "Eugene", "Frank", "Gary", "George", "Gerald",
	"Gregory", "Harold", "Henry", "Howard", "Jack", "James", "Jason",
	"Jeffrey", "Jeremy", "Jesse", "John", "Jonathan", "Joseph", "Joshua",
	"Justin", "Keith", "Kenneth", "Kevin", "Larry", "Lawrence", "Louis",
	"Marcus", "Mark", "Martin", "Matthew", "Michael", "Nathan", "Nicholas",
	"Oscar", "Patrick", "Paul", "Peter", "Philip", "Ralph", "Raymond",
	"Richard", "Robert", "Roger", "Ronald", "Roy", "Russell", "Ryan",
	"Samuel", "Scott", "Sean", "Stephen", "Steven", "Thomas", "Timothy",
	"Victor", "Vincent", "Walter", "Wayne", "William", "Zachary",
}

// FemaleGivenNames is a curated list of female first names.
var FemaleGivenNames = []string{
	"Abigail", "Alice", "Amanda", "Amy", "Andrea", "Angela", "Anna",
	"Barbara", "Betty", "Beverly", "Brenda", "Carol", "Carolyn", "Catherine",
	"Charlotte", "Christina", "Christine", "Cynthia", "Deborah", "Denise", "Diana",
	"Diane", "Dorothy", "Elizabeth", "Emily", "Emma", "Frances", "Gloria",
	"Grace", "Hannah", "Heather", "Helen", "Isabella", "Jacqueline", "Janet",
	"Janice", "Jean", "Jennifer", "Jessica", "Joan", "Joyce", "Judith",
	"Julia", "Julie", "Karen", "Katherine", "Kathleen", "Kathryn", "Kelly",
	"Kimberly", "Laura", "Lauren", "Linda", "Lisa", "Lori", "Louise",
	"Madison", "Margaret", "Maria", "Marie", "Marilyn", "Martha", "Mary",
	"Megan", "Melissa", "Michelle", "Nancy", "Nicole", "Olivia", "Pamela",
	"Patricia", "Rachel", "Rebecca", "Rose", "Ruth", "Samantha", "Sandra",
	"Sara", "Sarah", "Sharon", "Shirley", "Sophia", "Stephanie", "Susan",
	"Teresa", "Theresa", "Tiffany", "Virginia", "Wanda", "Wendy",
}

// MiddleNames is a list of middle names (can be used for either gender).
var MiddleNames = []string{
	"Alan", "Anne", "Benjamin", "Claire", "David", "Edward", "Elizabeth",
	"Frances", "Grace", "Henry", "James", "Jean", "John", "Joseph",
	"Katherine", "Lee", "Louise", "Lynn", "Mae", "Margaret", "Marie",
	"Michael", "Patricia", "Paul", "Ray", "Robert", "Rose", "Scott",
	"Thomas", "William",
}

// BloodTypes and their approximate distribution in the US population.
var BloodTypes = []struct {
	Type   string
	Weight int // Relative frequency (out of 1000)
}{
	{"O+", 374},
	{"A+", 316},
	{"B+", 102},
	{"O-", 67},
	{"A-", 63},
	{"AB+", 34},
	{"B-", 25},
	{"AB-", 19},
}

// Departments for vocation assignment.
var Departments = []string{
	"ENGINEERING",
	"MEDICAL",
	"SECURITY",
	"FOOD_PRODUCTION",
	"ADMINISTRATION",
	"EDUCATION",
	"SANITATION",
	"RESEARCH",
}

// DepartmentVocations maps departments to their vocations.
var DepartmentVocations = map[string][]struct {
	Code        string
	Title       string
	Clearance   int
	HazardLevel string
}{
	"ENGINEERING": {
		{"ENG-MAINT-01", "Maintenance Technician", 2, "MODERATE"},
		{"ENG-POWER-01", "Power Plant Operator", 4, "HIGH"},
		{"ENG-HVAC-01", "HVAC Technician", 2, "LOW"},
		{"ENG-WATER-01", "Water Treatment Specialist", 3, "MODERATE"},
		{"ENG-ELEC-01", "Electrician", 2, "MODERATE"},
		{"ENG-MECH-01", "Mechanical Engineer", 4, "LOW"},
	},
	"MEDICAL": {
		{"MED-PHYS-01", "Physician", 5, "LOW"},
		{"MED-NURS-01", "Nurse", 3, "LOW"},
		{"MED-TECH-01", "Medical Technician", 2, "LOW"},
		{"MED-SURG-01", "Surgeon", 6, "MODERATE"},
		{"MED-PSYC-01", "Psychologist", 4, "NONE"},
		{"MED-PHAR-01", "Pharmacist", 3, "LOW"},
	},
	"SECURITY": {
		{"SEC-GUAR-01", "Security Guard", 2, "MODERATE"},
		{"SEC-OFCR-01", "Security Officer", 4, "MODERATE"},
		{"SEC-CHIF-01", "Security Chief", 6, "MODERATE"},
		{"SEC-ARMO-01", "Armory Specialist", 4, "MODERATE"},
	},
	"FOOD_PRODUCTION": {
		{"FOOD-FARM-01", "Hydroponics Farmer", 1, "NONE"},
		{"FOOD-CHEF-01", "Chef", 2, "NONE"},
		{"FOOD-PROC-01", "Food Processor", 1, "LOW"},
		{"FOOD-NUTR-01", "Nutritionist", 3, "NONE"},
		{"FOOD-STOR-01", "Quartermaster", 3, "NONE"},
	},
	"ADMINISTRATION": {
		{"ADM-CLRK-01", "Clerk", 1, "NONE"},
		{"ADM-ACCT-01", "Accountant", 3, "NONE"},
		{"ADM-ASST-01", "Administrative Assistant", 2, "NONE"},
		{"ADM-DPTH-01", "Department Head", 6, "NONE"},
		{"ADM-OVSR-01", "Overseer", 10, "NONE"},
	},
	"EDUCATION": {
		{"EDU-TCHR-01", "Teacher", 2, "NONE"},
		{"EDU-LIBR-01", "Librarian", 2, "NONE"},
		{"EDU-COUN-01", "Counselor", 3, "NONE"},
		{"EDU-CHLD-01", "Childcare Specialist", 2, "NONE"},
	},
	"SANITATION": {
		{"SAN-WORK-01", "Sanitation Worker", 1, "MODERATE"},
		{"SAN-WAST-01", "Waste Management Technician", 2, "HIGH"},
		{"SAN-RECY-01", "Recycling Specialist", 1, "LOW"},
	},
	"RESEARCH": {
		{"RES-SCIE-01", "Research Scientist", 5, "MODERATE"},
		{"RES-TECH-01", "Lab Technician", 3, "MODERATE"},
		{"RES-ARCH-01", "Archivist", 3, "NONE"},
	},
}

// QuartersSectors defines the living quarters sectors.
var QuartersSectors = []string{"A", "B", "C", "D"}

// QuartersLevels defines the number of levels per sector.
const QuartersLevels = 5

// QuartersPerLevel defines units per level.
const QuartersPerLevel = 25

// ResourceCategories defines the resource categories for seeding.
var ResourceCategories = []struct {
	Code          string
	Name          string
	Description   string
	UnitOfMeasure string
	IsConsumable  bool
	IsCritical    bool
}{
	{"FOOD", "Food Supplies", "All edible provisions and meal components", "kg", true, true},
	{"WATER", "Water Supply", "Potable water for consumption and sanitation", "liters", true, true},
	{"MEDICAL", "Medical Supplies", "Medications, equipment, and medical consumables", "units", true, true},
	{"POWER", "Power Components", "Fuel cells, batteries, and power generation parts", "units", false, true},
	{"PARTS", "Spare Parts", "Mechanical and electrical components for repairs", "units", false, false},
	{"CLOTHING", "Clothing & Textiles", "Vault suits, uniforms, and fabric materials", "units", false, false},
	{"TOOLS", "Tools & Equipment", "Hand tools, power tools, and maintenance equipment", "units", false, false},
	{"CHEMICALS", "Chemicals", "Cleaning agents, industrial chemicals, and compounds", "liters", true, false},
}

// ResourceItems defines the resource items for seeding.
var ResourceItems = []struct {
	CategoryCode    string
	ItemCode        string
	Name            string
	Description     string
	UnitOfMeasure   string
	CaloriesPerUnit float64
	ShelfLifeDays   int
	IsProducible    bool
	ProdRatePerDay  float64
}{
	// Food items
	{"FOOD", "FOOD-PROTEIN-001", "Protein Rations", "Processed protein supplement bars", "kg", 3500, 365, true, 50},
	{"FOOD", "FOOD-CARBS-001", "Carbohydrate Mix", "Dehydrated carbohydrate powder", "kg", 3800, 730, true, 100},
	{"FOOD", "FOOD-VEGET-001", "Hydroponic Vegetables", "Fresh vegetables from hydroponics bay", "kg", 250, 7, true, 200},
	{"FOOD", "FOOD-FRUIT-001", "Preserved Fruits", "Canned and dehydrated fruits", "kg", 450, 365, false, 0},
	{"FOOD", "FOOD-MEALS-001", "Pre-packaged Meals", "Complete MRE-style meals", "units", 1200, 1825, false, 0},
	{"FOOD", "FOOD-SUGAR-001", "Sugar Compound", "Refined sugar substitute", "kg", 4000, 1095, true, 25},

	// Water
	{"WATER", "WATER-PURIF-001", "Purified Water", "Filtered and treated drinking water", "liters", 0, 0, true, 5000},
	{"WATER", "WATER-RECYC-001", "Recycled Gray Water", "Treated water for non-potable use", "liters", 0, 0, true, 10000},

	// Medical
	{"MEDICAL", "MED-STIM-001", "Stimpack", "Emergency healing compound", "units", 0, 1825, false, 0},
	{"MEDICAL", "MED-RADX-001", "Rad-X", "Radiation resistance medication", "units", 0, 3650, false, 0},
	{"MEDICAL", "MED-RADY-001", "RadAway", "Radiation purging compound", "units", 0, 1825, false, 0},
	{"MEDICAL", "MED-BAND-001", "Bandages", "Sterile medical bandages", "units", 0, 1825, true, 50},
	{"MEDICAL", "MED-ANTIBI-001", "Antibiotics", "General purpose antibiotic tablets", "units", 0, 730, false, 0},
	{"MEDICAL", "MED-SURG-001", "Surgical Supplies", "Sterile surgical equipment packs", "units", 0, 1095, false, 0},

	// Power
	{"POWER", "PWR-FCELL-001", "Fusion Cell", "Standard fusion power cell", "units", 0, 0, false, 0},
	{"POWER", "PWR-BATT-001", "Power Battery", "Rechargeable power storage unit", "units", 0, 0, true, 2},
	{"POWER", "PWR-CORE-001", "Reactor Core Element", "Nuclear reactor fuel element", "units", 0, 0, false, 0},

	// Parts
	{"PARTS", "PARTS-ELEC-001", "Electronic Components", "Assorted circuits and chips", "units", 0, 0, false, 0},
	{"PARTS", "PARTS-MECH-001", "Mechanical Parts", "Gears, bearings, and fasteners", "units", 0, 0, false, 0},
	{"PARTS", "PARTS-PIPE-001", "Piping Components", "Pipes, valves, and fittings", "units", 0, 0, false, 0},
	{"PARTS", "PARTS-FILT-001", "Air Filters", "HVAC and respirator filters", "units", 0, 365, true, 10},

	// Clothing
	{"CLOTHING", "CLOTH-SUIT-001", "Vault Suit", "Standard issue vault jumpsuit", "units", 0, 0, true, 5},
	{"CLOTHING", "CLOTH-BOOT-001", "Work Boots", "Standard issue footwear", "units", 0, 0, false, 0},
	{"CLOTHING", "CLOTH-UNDR-001", "Undergarments", "Basic clothing items", "units", 0, 0, true, 20},

	// Tools
	{"TOOLS", "TOOL-HAND-001", "Hand Tool Set", "Basic wrenches, screwdrivers, pliers", "units", 0, 0, false, 0},
	{"TOOLS", "TOOL-POWR-001", "Power Tools", "Drills, saws, and grinders", "units", 0, 0, false, 0},

	// Chemicals
	{"CHEMICALS", "CHEM-CLEAN-001", "Cleaning Solution", "Multi-purpose cleaning agent", "liters", 0, 365, true, 50},
	{"CHEMICALS", "CHEM-SANIT-001", "Sanitizer", "Antibacterial sanitizing solution", "liters", 0, 730, true, 25},
}
