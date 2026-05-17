# Small-Screen Layout - Analysis & Changes

## Obiettivo

Rendere l'interfaccia fruibile su smartphone (portrait, ~360x740). Breakpoint: `md` (768px). Sotto `md` = mobile layout; sopra = desktop invariato.

---

## File modificato

`web/src/routes/+page.svelte` - unico file che contiene header, pannelli Chat, Mappa e UDP stream.

---

## Problemi identificati

| Area | Problema |
|---|---|
| Header | Status pill "Connected" + "190 packets" occupano spazio prezioso |
| Layout pannelli | `flex-row` fisso: Chat e Mappa side-by-side, inutilizzabili su phone |
| Drag handles | Ridimensionamento pixel/% col mouse: irrilevante su touch |
| Chat typography | `text-sm` e `p-3` troppo grandi per 360px |
| UDP stream grid | `grid-cols-[4.5rem_1fr_2rem]` troppo larga; troppe colonne per riga |
| UDP campi secondari | Relays, destination, temp/hum/qnh, alt, hardware: overflow orizzontale |
| JSON button | Colonna da 32px occupata su ogni riga; inutile su touch |

---

## Soluzioni implementate

### 1. `isDesktop` reactive state

```ts
let isDesktop = $state(true);
$effect(() => {
  const mq = window.matchMedia('(min-width: 768px)');
  const update = () => (isDesktop = mq.matches);
  update();
  mq.addEventListener('change', update);
  return () => mq.removeEventListener('change', update);
});
```

Usato per applicare condizionalmente gli stili inline di drag (flex %, pixel height) solo su desktop.

### 2. Header

**Nascosto su mobile:**
- Separatore verticale `|`
- Status pill "● Connected"
- Contatore "190 packets"

**Aggiunto su mobile:**
- Pallino di stato minimale `md:hidden h-2 w-2 rounded-full {statusClass}` accanto al logo — indicatore visivo a colpo d'occhio senza testo.

```svelte
<!-- Mobile-only dot -->
<span class="md:hidden h-2 w-2 rounded-full {statusClass[connection]}"></span>
<!-- Desktop-only separator + full status -->
<div class="hidden md:block h-4 w-px bg-gray-600"></div>
<div class="hidden md:flex items-center gap-2 text-xs">...</div>
<!-- Packet counter: desktop only -->
<div class="hidden md:block font-mono text-xs text-gray-400">{events.length} packets</div>
```

### 3. Layout pannelli principali

**Prima:** `flex-row` fisso - Chat | Map side-by-side
**Dopo:** `flex-col md:flex-row` - su mobile impilati verticalmente: Chat -> Map -> UDP stream

```svelte
<div class="flex flex-col md:flex-row min-h-0 flex-1 gap-2" data-panel-row>
```

Chat e Map ricevono `min-h-[40vh]` su mobile per garantire altezza minima visibile. Gli stili inline `flex: 0 0 N%` e `flex: 1 1 0` sono condizionali a `isDesktop`.

**Drag handles nascosti su mobile** (`hidden md:flex`): ridimensionamento col mouse non ha senso su touch.

**UDP stream:** `min-h-[300px] md:min-h-[160px]`. Altezza pixel inline applicata solo su desktop:
```svelte
style={isDesktop ? `height: ${streamHeightPx}px` : ''}
```

### 4. Chat - riduzione tipografia

| Elemento | Prima | Dopo |
|---|---|---|
| Card padding | `p-3` | `p-2 md:p-3` |
| Gap header card | `gap-3` | `gap-2 md:gap-3` |
| Origin callsign | `text-sm` | `text-xs md:text-sm` |
| Timestamp | `text-[11px]` | `text-[10px] md:text-[11px]` |
| Message body | `text-sm` | `text-[13px] md:text-sm` |
| Footer meta (via/RSSI/SNR) | `text-[11px]` | `text-[10px] md:text-[11px]` |

### 5. UDP Stream - compressione su mobile

**Grid ridotta:** da 3 colonne a 2 su mobile (eliminata colonna JSON button):
```svelte
class="grid w-full grid-cols-[3rem_1fr] md:grid-cols-[4.5rem_1fr_2rem] ..."
```

**JSON button:** nascosto su mobile (`hidden md:flex`). Al suo posto, **tap sulla riga apre direttamente il modale JSON**:
```svelte
onclick={() => (isDesktop ? (selectedEvent = event) : (rawEvent = event))}
```

**Campi nascosti su mobile per tipo pacchetto:**

| Tipo | Nascosto su mobile | Visibile su mobile |
|---|---|---|
| `msg` | relays, arrow, destination | origin, msg text, RSSI/SNR |
| `tele` | relays, temp1, hum, qnh/qfe | origin, batt%, RSSI/SNR |
| `pos` | relays, alt, hardware | origin, lat/long, batt%, RSSI/SNR |
| fallback | relays, alt | origin, lat/long (se presenti), batt%, badge |

Pattern usato: `hidden md:inline` per `<span>` inline, `hidden md:flex` per `<span>` con `flex`.

### Comportamento desktop

Nessuna modifica visibile. Tutti gli stili desktop sono invariati:
- Drag handles presenti e funzionanti
- Chat %, Map `flex: 1 1 0`, UDP stream height pixel
- Tutti i campi UDP visibili
- JSON button visibile
- Status pill + packet counter visibili

The `Channels` sidebar now has a collapse button. When enabled, the left column shrinks into a narrow rail and the message list gets more horizontal space. State persists in `localStorage`.

### Verifica

```bash
cd web && npm run dev
```

Aprire Chrome DevTools -> Device Toolbar.

| Viewport | Atteso |
|---|---|
| 360x740 (phone) | Header: logo + callsign + mini dot. Pannelli impilati: Chat -> Map -> UDP. No drag handles. Campi UDP ridotti. Tap riga -> modale JSON. |
| 768x1024 (tablet) | Layout desktop completo |
| 1280x800 (desktop) | Identico a prima |

Resize live: layout si adatta senza jump. Stato drag preservato.
