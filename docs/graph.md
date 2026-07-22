# Graph View

The Graph page (`/graph`) renders a node-path tree from the same position data used by the map and Nodes table.

The root node is the active station callsign (`MyCall`). Each position record contributes one inbound path:

- Direct position packet: `MyCall -> ORIGIN`
- Relayed position packet `ORIGIN,R1,R2`: `MyCall -> R2 -> R1 -> ORIGIN`

The order is reversed from the packet source path because the graph is shown from the local station outward: the last relay in `via` is the direct radio neighbor, then earlier relays, then the packet origin.

The page uses SVG generated in Svelte and does not require Chart.js or extra graph dependencies.
When both endpoints have known coordinates, each highlighted link shows the distance between those two nodes. Known cumulative distance from `MyCall` adds vertical spacing on top of hop depth, so each hop stays roughly proportional to its measured distance. Sibling subtrees are spread horizontally with a minimum per-level gap so branches and first-hop nodes stay readable as the graph grows.

Graph nodes use the same freshness convention as the Nodes view:

- green: direct;
- blue: active indirect/normal;
- grey: old/stale;
- hidden: not shown.

Large relay trees show two hops by default so wide third-hop fan-outs do not shrink the whole graph. The toolbar can switch the visible hop limit between `2`, `3`, and `all`.

Interaction:

- Mouse wheel zooms around the cursor.
- Dragging pans the graph.
- Toolbar buttons zoom in, zoom out, and fit the graph back to the page.
- Hop buttons choose how many levels are visible.
- The `old` toggle shows or hides grey stale nodes.
- Selecting a node keeps its path highlighted and shows a semi-transparent summary panel. The panel lists each `MyCall -> selected node` path and its total distance when all path segments have coordinates.

## Map Graph Paths

The Map toolbar includes a Graph button that overlays active node paths on the OpenLayers map.

Only active non-grey nodes are included:

- direct nodes with a fresh `lastDirectSeen`;
- indirect nodes with a fresh `lastSeen`;
- stale grey nodes and hidden nodes are excluded.

Direct nodes draw `MyCall -> NODE`. Relayed nodes draw the inbound path in map order, for example `MyCall -> R2 -> R1 -> ORIGIN` for a packet source path `ORIGIN,R1,R2`.

When Graph mode is enabled, hovering a node highlights every active path up to five hops that can reach that node from `MyCall`. Interconnections are treated as usable in either direction for highlighting. Each highlighted path segment shows its distance label.
