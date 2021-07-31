/** @license ObjBrowse
 * Copyright 2021 The Go Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

import React, { useEffect, useState, useRef, useCallback } from "react";
import "bootstrap/dist/css/bootstrap.min.css";
import "./objbrowse.css";

import { useFetchJSON } from "./hooks";
import { Ranges } from "./ranges";

export type Entity = { type: "sym", id: number }

function entityKey(ent: Entity): string {
    return `${ent.type}/${ent.id}`;
}

export type Selection = { entity: Entity, ranges: Ranges }

export interface ViewProps {
    value: Selection;
    onSelect: (ent: Selection) => void;
    onSelectRange: (range: Ranges) => void;
}
export interface View {
    element: (props: ViewProps) => JSX.Element;
    id: string;
    label: string;
}

export interface AppProps { views: View[] }

type indexJSON = {
    Views: string[];
    Syms: { Names: string[], Views: number[] };
}

export function App(props: AppProps) {
    // TODO: Sync current entity (and selected range in that entity) to
    // the URL history.
    const [selected, setSelected] = useState<Selection | undefined>(undefined);

    const setEntity = useCallback((entity?: Entity) => {
        if (entity === undefined) {
            setSelected(undefined);
        } else {
            setSelected({ entity, ranges: new Ranges() });
        }
    }, [setSelected]);

    // Get object index.
    const indexJSON = useFetchJSON("/index");
    if (indexJSON.pending) {
        return <div className="d-flex justify-content-center align-items-center" style={{ height: "100vh" }}>
            {indexJSON.pending}
        </div>;
    }
    const index: indexJSON = indexJSON.value;

    // Map our views name to the server view indexes.
    //
    // TODO: Memoize this? Even changing the selected range re-renders
    // App. Tricky because of the indexJSON.pending branch. We could
    // move this between the useFetchJSON and the pending check, or make
    // a component that lives below App.
    let viewMap = new Map(props.views.map(view => [view.id, view]));
    let views: View[] = []; // By server index.
    for (let i = 0; i < index.Views.length; i++) {
        let view = viewMap.get(index.Views[i]);
        if (view !== undefined) {
            views[i] = view;
        }
    }

    // Compute the valid views for the selection.
    let validViews: View[] = [];
    if (selected !== undefined) {
        const viewSet = index.Syms.Views[selected.entity.id];
        for (let i = 0; i < views.length; i++) {
            if (views[i] && (viewSet & (1 << i))) {
                validViews.push(views[i]);
            }
        }
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
                        <SymPanel index={index} entity={selected?.entity} onSelectEntity={setEntity} />
                    </div>
                    {selected !== undefined &&
                        <div className="col-10 p-0">
                            <EntityPanel key={entityKey(selected.entity)} views={validViews} value={selected} onSelect={setSelected} />
                        </div>
                    }
                </div>
            </div>
        </div>
    );
}

interface SymPanelProps {
    index: indexJSON,
    entity?: Entity;
    onSelectEntity: (ent?: Entity) => void;
}

function SymPanel(props: SymPanelProps) {
    const [filterStr, setFilterStr] = useState("");
    const [filter, setFilter] = useState(() => new RegExp(filterStr));
    const [isError, setError] = useState(false);

    // TODO: Updating the filter can be rather slow with a large symbol list.
    // Consider updating after a short delay, or, ideally, reduce the cost of
    // the symbol list (e.g., with a lazy table).

    function handleChange(val: string) {
        setFilterStr(val);
        try {
            setFilter(new RegExp(val));
            setError(false);
        } catch (e) {
            setError(true);
        }
    }

    // TODO: Filter by address.
    // TODO: Filter by symbol type?

    return (<div className="ob-sympanel bg-light">
        <div className="m-3">
            <input value={filterStr} onChange={(ev) => handleChange(ev.target.value)} placeholder="Symbol regexp" className={isError ? "bg-warning" : ""}></input>
        </div>
        {
            // Don't add left or right margin. The SymList rows add their own.
        }
        <div className="my-3">
            <SymList index={props.index} filter={filter} entity={props.entity} onSelect={props.onSelectEntity} />
        </div>
    </div >);
}

interface SymListProps {
    index: indexJSON;
    filter: RegExp;
    entity?: Entity;
    onSelect: (ent?: Entity) => void;
}

function SymList(props: SymListProps) {
    // Scroll when the selected entity changes.
    const selectedElt = useRef<HTMLLIElement>(null);
    useEffect(() => {
        if (selectedElt.current !== null) {
            selectedElt.current.scrollIntoView({ block: "nearest" });
        }
    }, [props.entity]);

    // TODO: Indicate symbol type? Perhaps to the left of the symbol?
    //
    // TODO: Display columns with much more symbol information? I would probably
    // have to limit the symbol name length and make the SymPanel expandable.
    return (
        <ul className="ob-symlist list-unstyled text-nowrap">
            {props.index.Syms.Names.map((name, id) => {
                if (props.filter.test(name)) {
                    let extra = null;
                    if (props.entity?.type == "sym" && props.entity.id == id) {
                        extra = { className: "ob-symlist-selected", ref: selectedElt };
                    }
                    return <li key={id} onClick={() => props.onSelect({ type: "sym", id: id })} {...extra}><div>{name}</div></li>;
                }
            })}
        </ul >
    );
}

interface EntityPanelProps {
    views: View[];
    value: Selection;
    onSelect: (sel: Selection) => void;
}

function EntityPanel(props: EntityPanelProps) {
    const [viewID, setViewID] = useState(props.views[0].id);

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
            </nav>
            {/* The outer div fills the space */}
            <div>
                {props.views.map((view) =>
                    <EntityView key={view.id}
                        view={view} current={view.id == viewID} value={props.value}
                        onSelect={props.onSelect} onSelectRange={onSelectRange} />
                )}
            </div>
        </div>
    );
}

interface EntityViewProps extends ViewProps {
    view: View;
    current: boolean;
}

function EntityView(props: EntityViewProps) {
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
        const parent = domRef.current;
        if (parent !== null) {
            const selection = parent.querySelectorAll(".ob-selected");
            scrollToAll(parent, selection);
        }
    }, [props.value]);

    const View = props.view;
    // The ob-entity-view div controls visibility and creates a separate
    // scroll region this view. We use visibility with absolute
    // positioning instead of just display:none because we can't scroll
    // the contents of a display:none block.
    return (
        <div ref={domRef} className="ob-entity-view" style={{ visibility: props.current ? "visible" : "hidden" }}>
            {/* The inner div creates padding within the scroll region */}
            <div className="p-3">
                <View.element value={props.value} onSelect={props.onSelect} onSelectRange={props.onSelectRange}></View.element>
            </div>
        </div>
    );
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
    // Center list.
    let target = listTop - (parentRect.top + parentRect.height / 2) + (listBot - listTop) / 2;
    // Unless that would scroll the top of list out of view.
    const margin = 10;
    target = Math.max(target, margin);

    parent.scrollTo({ top: target, behavior: "smooth" });
}
