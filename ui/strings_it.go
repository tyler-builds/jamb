package ui

import "fmt"

// italianStrings is the Italian pack, it. It translates the short words and phrases of
// the controls. Some of them are the words of gpui-component (Apache-2.0, see NOTICE),
// and the Windows terms for the window buttons. The messages with values that depend
// on the context, and the sentences, stay English (see Strings).
func italianStrings() *Strings {
	return &Strings{
		Avatar:               "Avatar",
		Breadcrumb:           "Percorso di navigazione",
		Pagination:           "Paginazione",
		Navigation:           "Navigazione",
		Options:              "Opzioni",
		Suggestions:          "Suggerimenti",
		Times:                "Orari",
		Actions:              "Azioni",
		Color:                "Colore",
		Progress:             "Progresso",
		Loading:              "Caricamento",
		Notifications:        "Notifiche",
		ResizePanes:          "Ridimensiona riquadri",
		DateField:            "Data",
		DateRangeField:       "Intervallo di date",
		Steps:                "Passaggi",
		LoadingEllipsis:      "Caricamento…",
		LoadingSuggestions:   "Caricamento dei suggerimenti",
		SaturationBrightness: "Saturazione e luminosità",
		Hue:                  "Tonalità",
		Opacity:              "Opacità",
		Hex:                  "Esadecimale",
		ChooseDate:           "Scegli data",
		ChooseDates:          "Scegli intervallo di date",
		ShowTimes:            "Mostra orari",
		ShowHiddenLocations:  "Mostra posizioni nascoste",
		CollapseNavigation:   "Comprimi navigazione",
		ExpandNavigation:     "Espandi navigazione",
		DismissNotification:  "Chiudi notifica",
		PreviousPage:         "Pagina precedente",
		NextPage:             "Pagina successiva",
		PreviousYear:         "Anno precedente",
		PreviousMonth:        "Mese precedente",
		NextMonth:            "Mese successivo",
		NextYear:             "Anno successivo",
		Close:                "Chiudi",
		Minimize:             "Riduci a icona",
		Maximize:             "Ingrandisci",
		Restore:              "Ripristina",
		Increase:             "Aumenta",
		Decrease:             "Diminuisci",

		PanelGroup:      "Pannelli",
		MovePanelLeft:   "Sposta a sinistra",
		MovePanelRight:  "Sposta a destra",
		MovePanelUp:     "Sposta su",
		MovePanelDown:   "Sposta giù",
		SplitPanelRight: "Dividi a destra",
		SplitPanelDown:  "Dividi in basso",
		ClosePanel:      "Chiudi pannello",

		Remove:        func(tag string) string { return "Rimuovi " + tag },
		ResizeBetween: func(first, second string) string { return "Ridimensiona " + first + " e " + second },
		Page:          func(n int) string { return fmt.Sprintf("Pagina %d", n) },
		CurrentPage:   func(n int) string { return fmt.Sprintf("Pagina %d, pagina corrente", n) },
		PageOf:        func(page, pages int) string { return fmt.Sprintf("Pagina %d di %d", page, pages) },

		Today:        "Oggi",
		StartOfRange: "Inizio intervallo",
		EndOfRange:   "Fine intervallo",
		InRange:      "Nell'intervallo",

		NoResults:               "Nessun risultato",
		NoRows:                  "Nessuna riga",
		CouldNotLoadSuggestions: "Impossibile caricare i suggerimenti",
		SuggestionCount: func(n int) string {
			if n == 1 {
				return "1 suggerimento"
			}
			return fmt.Sprintf("%d suggerimenti", n)
		},
		SaturationBrightnessValue: func(saturation, brightness int) string {
			return fmt.Sprintf("Saturazione %d%%, luminosità %d%%", saturation, brightness)
		},
		SortedAscending:  "ordinato in ordine crescente",
		SortedDescending: "ordinato in ordine decrescente",

		CodeEditorFind:       "Trova",
		CodeEditorReplace:    "Sostituisci",
		CodeEditorReplaceAll: "Sostituisci tutto",
		CodeEditorNext:       "Successivo",
		CodeEditorPrevious:   "Precedente",
		CodeEditorMatchCase:  "Maiuscole/minuscole",
		CodeEditorWholeWord:  "Parola intera",
		CodeEditorRegex:      "Espressione regolare",
		CodeEditorGoToLine:   "Numero di riga",
		CodeEditorNoMatches:  "Nessun risultato",
		CodeEditorCount: func(current, total int) string {
			return fmt.Sprintf("%d di %d", current, total)
		},

		Required: "Questo campo è obbligatorio.",
	}
}
