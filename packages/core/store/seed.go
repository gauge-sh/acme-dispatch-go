package store

import (
	"fmt"
	"time"

	"github.com/acme-dispatch/dispatch/core"
)

// Seed wipes the database and loads the fixed development dataset. It goes
// through the same store methods as live traffic, so everything it creates
// obeys the domain rules, and the invoice totals fall out of the real
// pricing engine. The data is fixed, not random: running it twice gives the
// same rows and the same totals.
func (s *Store) Seed() error {
	return s.InTx(func(tx *Store) error {
		if err := tx.wipe(); err != nil {
			return err
		}
		if err := tx.seedUsers(); err != nil {
			return err
		}
		customers, err := tx.seedCustomers()
		if err != nil {
			return err
		}
		techs, err := tx.seedTechnicians()
		if err != nil {
			return err
		}
		parts, err := tx.seedParts()
		if err != nil {
			return err
		}
		return tx.seedJobs(customers, techs, parts)
	})
}

func (s *Store) wipe() error {
	tables := []string{
		"invoice_line_items", "invoice_records", "job_parts", "stock_adjustments",
		"appointments", "jobs", "stock_levels", "parts", "technicians", "customers",
		"api_tokens", "users",
	}
	for _, table := range tables {
		if _, err := s.exec("DELETE FROM " + table); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) seedUsers() error {
	admin := core.User{Email: "admin@acme-dispatch.example", Name: "Alex Morgan", Role: core.RoleAdmin, IsActive: true}
	if err := s.CreateUser(&admin, "dispatch-admin"); err != nil {
		return err
	}
	office := core.User{Email: "office@acme-dispatch.example", Name: "Sam Whitaker", Role: core.RoleDispatcher, IsActive: true}
	return s.CreateUser(&office, "dispatch-office")
}

func (s *Store) seedCustomers() ([]core.Customer, error) {
	rows := []struct{ name, email, phone, address, notes string }{
		{"Harbor Coffee Roasters", "ops@harborcoffee.example", "555-0101", "12 Pier Rd", "walk-in cooler is behind the counter"},
		{"Northside Dental", "office@northsidedental.example", "555-0102", "230 Elm Ave Suite 4", ""},
		{"Cliffside Inn", "maint@cliffsideinn.example", "555-0103", "1 Overlook Dr", "call ahead, gate code 4412"},
		{"Petra's Bakery", "petra@petrasbakery.example", "555-0104", "88 Mill St", ""},
		{"Lakeview Apartments", "super@lakeviewapts.example", "555-0105", "400 Lakeview Blvd", "12 units, boiler in basement"},
		{"Grant Street Gym", "", "555-0106", "77 Grant St", ""},
		{"Two Rivers Brewing", "brewhouse@tworivers.example", "555-0107", "5 Confluence Way", "glycol chiller on mezzanine"},
		{"Maple & Main Diner", "", "555-0108", "101 Main St", ""},
		{"Eastgate Pharmacy", "rx@eastgate.example", "555-0109", "310 Eastgate Mall", "vaccine fridge is priority"},
		{"Sunrise Daycare", "director@sunrisekids.example", "555-0110", "9 Meadow Ln", "no work during nap hours 12-2"},
		{"Holloway & Sons Auto", "shop@holloway.example", "555-0111", "48 Industrial Pkwy", ""},
		{"Riverbend Library", "facilities@riverbend.example", "555-0112", "200 River Rd", ""},
		{"The Corner Florist", "", "555-0113", "14 Main St", ""},
		{"Summit Physical Therapy", "admin@summitpt.example", "555-0114", "620 Summit Ave", ""},
		{"Old Mill Antiques", "info@oldmill.example", "555-0115", "3 Mill Race Ct", "humidity control matters, antiques"},
		{"Bright Path Charter School", "ops@brightpath.example", "555-0116", "55 Scholar Way", "check in at front office"},
		{"Casa Verde Restaurant", "manager@casaverde.example", "555-0117", "212 Verde Plaza", ""},
		{"Pinecrest Veterinary", "frontdesk@pinecrestvet.example", "555-0118", "80 Pinecrest Rd", "kennel exhaust fan is loud"},
		{"Fairfield Storage", "", "555-0119", "1500 Fairfield Ext", "units 40-60 on second floor"},
		{"Beacon Hill Bed & Breakfast", "stay@beaconhillbb.example", "555-0120", "7 Beacon Hill Ln", ""},
	}
	customers := make([]core.Customer, 0, len(rows))
	for _, r := range rows {
		c := core.Customer{Name: r.name, ServiceAddress: r.address}
		if r.email != "" {
			c.ContactEmail = &r.email
		}
		if r.phone != "" {
			c.Phone = &r.phone
		}
		if r.notes != "" {
			c.Notes = &r.notes
		}
		if err := s.CreateCustomer(&c); err != nil {
			return nil, fmt.Errorf("customer %s: %w", r.name, err)
		}
		customers = append(customers, c)
	}
	return customers, nil
}

func (s *Store) seedTechnicians() ([]core.Technician, error) {
	rows := []struct {
		name, phone string
		rate        int64
		active      bool
		skills      []string
	}{
		{"Dana Reyes", "555-0201", 8000, true, []string{"hvac", "refrigeration"}},
		{"Marcus Webb", "555-0202", 6500, true, []string{"electrical"}},
		{"Iris Kowalski", "555-0203", 7200, true, []string{"plumbing", "boilers"}},
		{"Theo Brandt", "555-0204", 5000, true, []string{"hvac"}},
		{"Priya Raman", "555-0205", 7800, true, []string{"electrical", "controls"}},
		{"Cole Hutchins", "555-0206", 5500, true, []string{"plumbing"}},
		{"Yuki Tanaka", "555-0207", 8400, true, []string{"refrigeration", "controls"}},
		{"Omar Haddad", "555-0208", 6000, false, []string{"hvac", "plumbing"}},
	}
	techs := make([]core.Technician, 0, len(rows))
	for _, r := range rows {
		t := core.Technician{Name: r.name, HourlyCostCents: r.rate, IsActive: r.active, SkillTags: r.skills}
		phone := r.phone
		t.Phone = &phone
		if err := s.CreateTechnician(&t); err != nil {
			return nil, fmt.Errorf("technician %s: %w", r.name, err)
		}
		techs = append(techs, t)
	}
	return techs, nil
}

func (s *Store) seedParts() ([]core.Part, error) {
	rows := []struct {
		sku, name   string
		cost, price int64
		onHand      int64
	}{
		{"IGN-4420", "Hot surface igniter", 800, 1299, 14},
		{"FLT-16255", "Pleated filter 16x25x5", 450, 899, 60},
		{"CAP-455", "Run capacitor 45/5 uF", 620, 1150, 22},
		{"MTR-3305", "Condenser fan motor 1/3 HP", 4800, 7900, 6},
		{"TXV-R410", "Thermal expansion valve R-410A", 3900, 6400, 8},
		{"CNT-2P30", "Contactor 2-pole 30A", 950, 1675, 18},
		{"THS-724", "Programmable thermostat", 3200, 5450, 11},
		{"PMP-CND1", "Condensate pump", 2700, 4300, 9},
		{"BRD-HSI90", "Furnace control board", 6800, 10900, 4},
		{"BLW-12X9", "Blower wheel 12x9", 3100, 5200, 5},
		{"VLV-GAS34", "Gas valve 3/4 in", 5400, 8600, 7},
		{"SEN-FLM2", "Flame sensor", 550, 1050, 25},
		{"REL-SPDT", "Fan relay SPDT", 700, 1225, 20},
		{"DUC-FLX8", "Flex duct 8 in x 25 ft", 2900, 4700, 10},
		{"PIP-CPV34", "CPVC pipe 3/4 in x 10 ft", 600, 1100, 40},
		{"WTR-HTR50", "Water heater element 5000W", 1400, 2350, 12},
		{"ANO-ROD44", "Anode rod 44 in", 1800, 2999, 15},
		{"EXP-TNK2", "Expansion tank 2 gal", 3300, 5300, 6},
		{"ZNE-VLV1", "Zone valve 1 in", 4100, 6700, 8},
		{"CIR-PMP07", "Circulator pump 007", 5600, 9200, 5},
		{"BRK-20A", "Circuit breaker 20A", 850, 1500, 30},
		{"GFI-15A", "GFCI outlet 15A", 1100, 1900, 24},
		{"WIR-12G50", "12-gauge wire 50 ft", 2300, 3800, 16},
		{"LGT-LED4", "LED shop light 4 ft", 1900, 3200, 13},
		{"SMK-COMBO", "Smoke/CO combo detector", 2100, 3600, 19},
	}
	parts := make([]core.Part, 0, len(rows))
	for _, r := range rows {
		p := core.Part{SKU: r.sku, Name: r.name, UnitCostCents: r.cost, UnitPriceCents: r.price}
		if err := s.CreatePart(&p); err != nil {
			return nil, fmt.Errorf("part %s: %w", r.sku, err)
		}
		if r.onHand > 0 {
			reason := "initial receiving"
			if _, err := s.AdjustStockLevel(p.ID, r.onHand, &reason); err != nil {
				return nil, err
			}
		}
		parts = append(parts, p)
	}
	return parts, nil
}

var seedJobTitles = []string{
	"Furnace not igniting",
	"AC blowing warm air",
	"Quarterly filter service",
	"Water heater pilot keeps going out",
	"Walk-in cooler over temp",
	"Panel upgrade quote follow-up",
	"Thermostat replacement",
	"Leak under prep sink",
	"Rooftop unit annual service",
	"Boiler pressure dropping",
	"Exhaust fan rattling",
	"Freezer compressor short-cycling",
	"No hot water in unit 4",
	"Breaker trips when heat starts",
	"Condensate line clogged",
	"Zone valve stuck closed",
	"Install smoke detectors",
	"Duct reroute in back office",
	"GFCI outlets dead in kitchen",
	"Emergency: no heat",
}

// seedJobs walks 40 jobs to a spread of statuses. Appointment slots are laid
// out so no technician is double-booked: mornings 9-11, afternoons 13-15,
// one job per technician per day per slot.
func (s *Store) seedJobs(customers []core.Customer, techs []core.Technician, parts []core.Part) error {
	monday := time.Date(2025, 6, 2, 0, 0, 0, 0, time.UTC)
	// status plan: 8 requested, 8 scheduled, 6 in_progress, 6 completed,
	// 7 invoiced, 5 cancelled
	plan := make([]core.JobStatus, 0, 40)
	add := func(st core.JobStatus, n int) {
		for range n {
			plan = append(plan, st)
		}
	}
	add(core.StatusRequested, 8)
	add(core.StatusScheduled, 8)
	add(core.StatusInProgress, 6)
	add(core.StatusCompleted, 6)
	add(core.StatusInvoiced, 7)
	add(core.StatusCancelled, 5)

	activeTechs := techs[:7] // the eighth is inactive and takes no new work

	for i, target := range plan {
		customer := customers[i%len(customers)]
		priority := core.PriorityNormal
		switch {
		case i%7 == 0:
			priority = core.PriorityUrgent
		case i%5 == 0:
			priority = core.PriorityLow
		}
		title := seedJobTitles[i%len(seedJobTitles)]
		desc := fmt.Sprintf("Reported by %s. See appointment notes for findings.", customer.Name)
		job := core.Job{CustomerID: customer.ID, Title: title, Priority: priority, Description: &desc}
		if err := s.CreateJob(&job); err != nil {
			return fmt.Errorf("job %d: %w", i, err)
		}

		if target == core.StatusRequested {
			continue
		}
		if target == core.StatusCancelled && i%2 == 0 {
			// Half the cancellations never got scheduled.
			if _, err := s.TransitionJob(job.ID, core.StatusCancelled); err != nil {
				return err
			}
			continue
		}

		tech := activeTechs[i%len(activeTechs)]
		day := monday.AddDate(0, 0, i/len(activeTechs))
		start := core.NewTime(day.Add(9 * time.Hour))
		end := core.NewTime(day.Add(11 * time.Hour))
		appt := core.Appointment{JobID: job.ID, TechnicianID: tech.ID, ScheduledStart: start, ScheduledEnd: end}
		if err := s.CreateAppointment(&appt); err != nil {
			return fmt.Errorf("job %d appointment: %w", i, err)
		}

		// Invoiced jobs get an afternoon follow-up; every third of those
		// lands on Saturday so weekend surcharges show up in real data.
		if target == core.StatusInvoiced {
			followTech := activeTechs[(i+3)%len(activeTechs)]
			followDay := day
			if i%3 == 0 {
				followDay = monday.AddDate(0, 0, 5) // Saturday of week one
			}
			follow := core.Appointment{
				JobID:          job.ID,
				TechnicianID:   followTech.ID,
				ScheduledStart: core.NewTime(followDay.Add(time.Duration(13+i%2) * time.Hour)),
				ScheduledEnd:   core.NewTime(followDay.Add(time.Duration(13+i%2) * time.Hour).Add(90 * time.Minute)),
			}
			if err := s.CreateAppointment(&follow); err != nil {
				return fmt.Errorf("job %d follow-up: %w", i, err)
			}
		}

		if _, err := s.TransitionJob(job.ID, core.StatusScheduled); err != nil {
			return err
		}
		if target == core.StatusScheduled {
			continue
		}
		if target == core.StatusCancelled {
			if _, err := s.TransitionJob(job.ID, core.StatusCancelled); err != nil {
				return err
			}
			continue
		}

		if _, err := s.TransitionJob(job.ID, core.StatusInProgress); err != nil {
			return err
		}
		// Worked jobs consume parts.
		for k := range 1 + i%3 {
			part := parts[(i*3+k*7)%len(parts)]
			if _, err := s.AddJobPart(job.ID, part.ID, int64(1+(i+k)%2)); err != nil {
				return fmt.Errorf("job %d part %s: %w", i, part.SKU, err)
			}
		}
		if target == core.StatusInProgress {
			continue
		}

		notes := "Completed and tested. Left site clean."
		appt.CompletionNotes = &notes
		if err := s.UpdateAppointment(&appt); err != nil {
			return err
		}
		if _, err := s.TransitionJob(job.ID, core.StatusCompleted); err != nil {
			return err
		}
		if target == core.StatusCompleted {
			continue
		}

		if _, err := s.CreateInvoiceRecord(job.ID); err != nil {
			return fmt.Errorf("job %d invoice: %w", i, err)
		}
	}
	return nil
}
