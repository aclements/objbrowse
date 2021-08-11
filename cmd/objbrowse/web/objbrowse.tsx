/** @license ObjBrowse
 * Copyright 2021 The Go Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

import React, { useEffect, useState, useRef, useCallback } from "react";
import "bootstrap/dist/css/bootstrap.min.css";
import "./objbrowse.css";

import { useFetchJSON } from "./hooks";
import { Ranges, Range } from "./ranges";
import * as History from "./history";

export type Entity = { type: "sym", id: number }

function entityKey(ent: Entity): string {
    return `${ent.type}/${ent.id}`;
}

export type Selection = { entity: Entity, ranges: Ranges }

function selectionToString(s: Selection | undefined): string {
    if (s === undefined) {
        return "";
    }
    let str = `${s.entity.type}/${s.entity.id}`;
    s.ranges.ranges.forEach((r, i) => {
        str += i == 0 ? "@" : ",";
        str += `${r.start.toString(16)}-${r.end.toString(16)}`;
    });
    return str;
}

function selectionFromString(str: string): Selection | undefined {
    const m = /^sym[/]([0-9]+)(?:@(.*))?$/.exec(str);
    if (m === null) {
        return undefined;
    }
    const entity: Entity = { type: "sym", id: parseInt(m[1]) };
    let ranges: Range[] = [];
    if (m[2] !== undefined) {
        for (let r of m[2].split(",")) {
            const rm = /^([0-9a-f]+)-([0-9a-f]+)$/i.exec(r);
            if (rm === null) {
                return undefined;
            }
            ranges.push({ start: BigInt("0x" + rm[1]), end: BigInt("0x" + rm[2]) });
        }
    }
    return { entity: entity, ranges: new Ranges(ranges) };
}

export interface ViewProps {
    value: Selection;
    onSelect: (ent: Selection, push?: History.PushKind) => void;
    onSelectRange: (range: Ranges) => void;
}
export interface View {
    element: (props: ViewProps) => JSX.Element;
    id: string;
    label: string;
}

export interface AppProps {
    views: View[];
    history: History.Key;
}

type indexJSON = {
    Views: string[];
    Syms: {
        Names: string[];
        Values: string[];
        Sizes: number[];
        Kinds: string;
        Views: number[];
    };
}

export function App(props: AppProps) {
    // Get object index.
    const indexJSON = useFetchJSON("/index");
    if (indexJSON.pending) {
        return <div className="d-flex justify-content-center align-items-center" style={{ height: "100vh" }}>
            {indexJSON.pending}
        </div>;
    }
    const index: indexJSON = indexJSON.value;

    // Map our views name to the server view indexes.
    let viewMap = new Map(props.views.map(view => [view.id, view]));
    let views: View[] = []; // By server index.
    for (let i = 0; i < index.Views.length; i++) {
        let view = viewMap.get(index.Views[i]);
        if (view !== undefined) {
            views[i] = view;
        }
    }

    // Un-compact symbol information.
    let syms: Sym[] = [];
    const is = index.Syms;
    for (let i = 0; i < index.Syms.Names.length; i++) {
        syms.push(new Sym(is.Names[i], BigInt("0x" + is.Values[i]), is.Sizes[i], is.Kinds[i], views, is.Views[i]));
    }

    return <App1 {...props} syms={syms} />;
}

class Sym {
    private _viewIndex: View[];
    private _viewMask: number; // Bitmask into _viewIndex.

    constructor(public name: string, public value: bigint, public size: number, public kind: string, viewIndex: View[], viewMask: number) {
        this._viewIndex = viewIndex;
        this._viewMask = viewMask;
    }

    get views() {
        let validViews: View[] = [];
        for (let i = 0; i < this._viewIndex.length; i++) {
            if (this._viewIndex[i] && (this._viewMask & (1 << i))) {
                validViews.push(this._viewIndex[i]);
            }
        }
        return validViews;
    }
}

interface App1Props extends AppProps {
    syms: Sym[];
}

function App1(props: App1Props) {
    const [selected, setSelected] = History.useState<Selection | undefined>(props.history, undefined, selectionToString, selectionFromString);

    const setEntity = useCallback((entity?: Entity) => {
        // Changing the whole entity always pushes a new history record.
        if (entity === undefined) {
            setSelected(undefined, "push");
        } else {
            setSelected({ entity, ranges: new Ranges() }, "push");
        }
    }, [setSelected]);

    // Get the valid views for the selection.
    let validViews: View[];
    if (selected === undefined) {
        validViews = [];
    } else {
        validViews = props.syms[selected.entity.id].views;
    }

    // The EntityPanel is keyed on the selected entity. This causes
    // React to rebuild the whole EntityPanel when the entity changes.
    // This makes sense because there's not much that makes sense to
    // reuse, and it re-evaluates the best valid view to show.
    return (
        <div className="ob-root">
            <div className="container-fluid">
                <div className="row flex-xl-nowrap">
                    <div className="col-2 p-0">
                        <SymPanel syms={props.syms} entity={selected?.entity} onSelectEntity={setEntity} />
                    </div>
                    {selected !== undefined &&
                        <div className="col-10 p-0 ob-entity-container">
                            <EntityPanel key={entityKey(selected.entity)} history={props.history.sub(entityKey(selected.entity))}
                                views={validViews} value={selected} onSelect={setSelected} />
                        </div>
                    }
                </div>
            </div>
        </div>
    );
}

interface SymPanelProps {
    syms: Sym[],
    entity?: Entity;
    onSelectEntity: (ent?: Entity) => void;
}

type SymFilter = (sym: Sym) => boolean;

function SymPanel(props: SymPanelProps) {
    const parseFilter = (str: string) => {
        if (str === "") {
            return (sym: Sym) => true;
        }
        // Try parsing as an address.
        let addr: undefined | bigint;
        if (/^0x[0-9a-fA-F]+$/.test(str)) {
            addr = BigInt(str);
        } else if (/^[0-9a-fA-F]+$/.test(str)) {
            addr = BigInt("0x" + str);
        }
        // Try parsing as a regexp.
        let regexp: undefined | RegExp;
        try {
            regexp = new RegExp(str);
        } catch (e) { }
        if (addr === undefined && regexp === undefined) {
            return undefined;
        }
        return (sym: Sym) => {
            return ((addr !== undefined && sym.value <= addr && addr - sym.value < sym.size) ||
                (regexp !== undefined && regexp.test(sym.name)));
        }
    };

    const [filterStr, setFilterStr] = useState("");
    // We box the filter function in a tuple because both useState and
    // setX treat function arguments specially.
    const [filter, setFilter] = useState<[SymFilter]>([(sym: Sym) => true]);
    const [isError, setError] = useState(false);

    // TODO: Updating the filter can be rather slow with a large symbol list.
    // Consider updating after a short delay, or, ideally, reduce the cost of
    // the symbol list (e.g., with a lazy table).

    function handleChange(val: string) {
        setFilterStr(val);
        let f = parseFilter(val);
        if (f === undefined) {
            setError(true);
        } else {
            setFilter([f]);
            setError(false);
        }
    }

    // Make "/" jump to the search box.
    const inputRef = useRef<HTMLInputElement>(null);
    useEffect(() => {
        function onkeydown(e: KeyboardEvent) {
            // Don't intercept keys on input elements.
            if (e.target instanceof Element &&
                (/input|select|textarea/i.test(e.target.nodeName))) {
                return;
            }
            if (e.key == "/") {
                if (inputRef.current !== null) {
                    inputRef.current.focus();
                    inputRef.current.select();
                    // Avoid typing "/" into the input.
                    e.preventDefault();
                }
            }
        }
        window.addEventListener("keydown", onkeydown);
        return () => { window.removeEventListener("keydown", onkeydown); };
    });

    // TODO: Filter by symbol type?

    return (<div className="ob-sympanel bg-light">
        <div className="m-3">
            <input ref={inputRef} value={filterStr} onChange={(ev) => handleChange(ev.target.value)} placeholder="Name regexp or hex address [/]" className={isError ? "bg-warning" : ""} />
        </div>
        {
            // Don't add left or right margin. The SymList rows add their own.
        }
        <div className="my-3">
            <SymList syms={props.syms} filter={filter[0]} entity={props.entity} onSelect={props.onSelectEntity} />
        </div>
    </div >);
}

interface SymListProps {
    syms: Sym[];
    filter: SymFilter;
    entity?: Entity;
    onSelect: (ent?: Entity) => void;
}

const SymList = React.memo(function SymList(props: SymListProps) {
    // Scroll when the selected entity changes.
    const domRef = useRef<HTMLUListElement>(null);
    useEffect(() => {
        // Scroll the current selection into view.
        const parent = domRef.current;
        if (parent !== null) {
            const selection = parent.querySelector(".ob-symlist-selected");
            if (selection !== null) {
                selection.scrollIntoView({ block: "nearest" });
            }
        }
    }, [domRef]);

    // TODO: Options to sort by name or address.

    // Use a single, memoized onClick handler for all symbols.
    const onClick = useCallback<React.MouseEventHandler<HTMLLIElement>>((ev) => {
        let id = parseInt(ev.currentTarget.dataset.id || "");
        props.onSelect({ type: "sym", id: id });
    }, [props.onSelect]);

    return (
        <ul ref={domRef} className="ob-symlist list-unstyled text-nowrap">
            {props.syms.map((sym, id) => {
                if (props.filter(sym)) {
                    let className = "";
                    if (props.entity?.type == "sym" && props.entity.id == id) {
                        className = "ob-symlist-selected";
                    }
                    let info;
                    if (sym.kind === 'U') {
                        // Undefined symbol. Value is not meaningful.
                        info = sym.kind;
                    } else {
                        info = `${sym.kind} ${sym.value.toString(16)}`;
                    }
                    return (<li key={id} onClick={onClick} data-id={id} className={className}>
                        <div><span className="ob-symlist-addr">{info}</span> {sym.name}</div>
                    </li>);
                }
            })}
        </ul >
    );
});

interface EntityPanelProps {
    views: View[];
    value: Selection;
    onSelect: (sel: Selection, push?: History.PushKind) => void;
    history: History.Key;
}

type EntityPanelAction = ["add"] | ["close", number]

/**
 * EntityPanel displays the selected entity as one or more
 * EntityColumns.
 */
function EntityPanel(props: EntityPanelProps) {
    type state = { id: number, panels: number[] };
    const decodeState = (x: any): state | undefined => {
        if (typeof x !== "string") {
            return undefined;
        }
        let panels = x.split(" ").map(v => parseInt(v));
        if (panels.some(v => isNaN(v))) {
            return undefined;
        }
        return { id: Math.max(...panels) + 1, panels };
    }
    const [state, dispatch] = History.useReducer(props.history,
        (old: state, action: EntityPanelAction) => {
            switch (action[0]) {
                case "add":
                    return { id: old.id + 1, panels: [...old.panels, old.id] };
                case "close":
                    return { id: old.id, panels: old.panels.filter(id => id != action[1]) };
            }
        },
        { id: 1, panels: [0] },
        v => v.panels.join(" "),
        decodeState);

    const panels = state.panels;
    return (<>
        {panels.map((id, idx) =>
            <EntityColumn key={id} {...props} history={props.history.sub(id.toString())}
                update={dispatch} id={id} many={panels.length > 1} last={idx == panels.length - 1} />)}
    </>);
}

interface EntityColumnProps extends EntityPanelProps {
    update: (action: EntityPanelAction) => void;
    id: number;
    many: boolean;
    last: boolean;
}

/**
 * EntityColumn displays the set of valid views for the current entity.
 */
const EntityColumn = React.memo(function EntityColumn(props: EntityColumnProps) {
    const [viewID, setViewID] = History.useState(props.history, props.views[0].id, v => v, x => typeof x === "string" ? x : undefined);

    const onSelectRange = useCallback((range: Ranges) => {
        props.onSelect({ entity: props.value.entity, ranges: range });
    }, [props.onSelect]);

    return (
        <div className="ob-entity-panel">
            {/* padding-left extends the bottom border to the left */}
            {/* padding-top keeps it in place when scrolling */}
            <nav className="nav nav-tabs ps-3 pt-3">
                {props.views.map((view) =>
                    view.id === viewID ?
                        <span key={view.id} className="nav-link active" aria-current="page">{view.label}</span> :
                        <span key={view.id} className="nav-link" onClick={() => setViewID(view.id)}>{view.label}</span>
                )}
                <span className="ob-entity-panel-buttons">
                    {props.many && <EntityNavButton type="close" title="Close column" onClick={() => props.update(["close", props.id])} />}
                    {props.last && <EntityNavButton type="add" title="New column" onClick={() => props.update(["add"])} />}
                </span>
            </nav>
            {/* The outer div fills the space */}
            <div>
                {props.views.map(View =>
                    // The ob-entity-view div controls visibility and creates a separate
                    // scroll region this view. We use visibility with absolute
                    // positioning instead of just display:none because we can't scroll
                    // the contents of a display:none block.
                    <div key={View.id} className="ob-entity-view" style={{ visibility: View.id == viewID ? "visible" : "hidden" }}>
                        {/* The inner div creates padding within the scroll region */}
                        <div className="p-3">
                            <View.element value={props.value} onSelect={props.onSelect} onSelectRange={onSelectRange}></View.element>
                        </div>
                    </div>
                )}
            </div>
        </div>
    );
});

function EntityNavButton(props: { type: "close" | "add", title?: string, onClick: () => void }) {
    let path;
    // For the paths, use a viewbox a little smaller than 0,0 10x10. The
    // actual viewbox is bigger so we can highlight on hover.
    if (props.type === "close") {
        path = <path d="M 2 2L8 8M2 8L8 2" stroke="#495057" strokeWidth="1.2" />;
    } else {
        path = <path d="M1 1V9H9V1ZM5 1V9" stroke="#495057" fill="none" />;
    }
    let svg = <svg width="1.6rem" height="1.6rem" viewBox="-3 -3 16 16" onClick={props.onClick}>{path}</svg>
    return <span role="button" className="ob-entity-nav-button" title={props.title}>{svg}</span>;
}

export function ViewScroller(props: { value: Selection, children: React.ReactChild }) {
    const domRef = useRef<HTMLDivElement>(null);
    useEffect(() => {
        // Scroll the current selection into view.
        //
        // We could just use scrollIntoView, but because of a Chrome
        // bug, we can only smooth scroll one element and we need to
        // scroll all of the views
        // (https://bugs.chromium.org/p/chromium/issues/detail?id=833617).
        // scrollTo doesn't have this problem, and gives us the
        // opportunity to scroll the whole selection into view and apply
        // a bit of custom logic.
        let parent: (HTMLElement | null) = domRef.current;
        // Find the ob-entity-view, which is the scrollable container.
        while (parent !== null && !parent.classList.contains("ob-entity-view")) {
            parent = parent.parentElement;
        }
        if (parent !== null) {
            const selection = parent.querySelectorAll(".ob-selected");
            scrollToAll(parent, selection);
        }
    }, [props.value]);
    return <div ref={domRef}>{props.children}</div>;
}

function scrollToAll(parent: Element, list: NodeListOf<Element>) {
    // Get the combined bounding rect of list.
    let listTop, listBot;
    for (let node of list) {
        const nodeRect = node.getBoundingClientRect();
        if (listTop === undefined || nodeRect.top < listTop) {
            listTop = nodeRect.top;
        }
        if (listBot === undefined || nodeRect.bottom > listBot) {
            listBot = nodeRect.bottom;
        }
    }
    if (listTop === undefined || listBot === undefined) {
        return;
    }

    // Compute how to scroll parent to show list.
    const parentRect = parent.getBoundingClientRect();
    // If the top of the list is already visible, don't do anything.
    if (parentRect.top <= listTop && listTop < parentRect.bottom) {
        return;
    }
    // Center list. Remember that listTop/listBot are relative to the
    // screen and thus relative to parent's current scroll offset, but
    // we need to compute a new absolute scroll offset.
    let target = parent.scrollTop + listTop - (parentRect.top + parentRect.height / 2) + (listBot - listTop) / 2;
    // Unless that would scroll the top of list out of view.
    const margin = 16;
    let limit = parent.scrollTop + listTop - (parentRect.top + margin);
    target = Math.min(target, limit);
    parent.scrollTo({ top: target, behavior: "smooth" });
}
