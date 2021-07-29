/** @license ObjBrowse
 * Copyright 2021 The Go Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

import React, { useEffect, useState, useRef } from "react";
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
    // TODO: Sync current entity (and selected range in that entity) to the URL
    // history.
    const [selected, setSelected] = useState<Selection | undefined>(undefined);
    const [validViews, setValidViews] = useState<View[]>([]);
    const [view, setView] = useState("");

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

    const setEntity = (entity?: Entity) => {
        let validViews: View[] = [];
        if (entity === undefined) {
            setSelected(undefined);
        } else {
            setSelected({ entity, ranges: new Ranges() });
            // Get valid views for this symbol.
            const viewSet = index.Syms.Views[entity.id];
            for (let i = 0; i < views.length; i++) {
                if (views[i] && (viewSet & (1 << i))) {
                    validViews.push(views[i]);
                }
            }
        }
        setValidViews(validViews);
        if (validViews.length > 0) {
            setView(validViews[0].id);
        } else {
            setView("");
        }
    }

    return (
        <div className="ob-root">
            <div className="container-fluid">
                <div className="row flex-xl-nowrap">
                    <div className="col-2 p-0">
                        <SymPanel index={index} entity={selected?.entity} onSelect={setEntity} />
                    </div>
                    {selected !== undefined &&
                        <div className="col-10 p-0">
                            <EntityPanel views={validViews} viewID={view} onSelectView={setView} value={selected} onSelect={setSelected} />
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
    onSelect: (ent?: Entity) => void;
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
            <SymList index={props.index} filter={filter} entity={props.entity} onSelect={props.onSelect} />
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
    viewID: string;
    onSelectView: (view: string) => void;
    value: Selection;
    onSelect: (sel: Selection) => void;
}

function EntityPanel(props: EntityPanelProps) {
    return (
        <div className="ob-view-container">
            {/* padding-left extends the bottom border to the left */}
            {/* padding-top keeps it in place when scrolling */}
            <nav className="nav nav-tabs ps-3 pt-3">
                {props.views.map((View) =>
                    View.id == props.viewID ?
                        <span key={View.id} className="nav-link active" aria-current="page">{View.label}</span> :
                        <span key={View.id} className="nav-link" onClick={() => props.onSelectView(View.id)}>{View.label}</span>
                )}
            </nav>
            {/* The outer div fills the space */}
            <div>
                {props.views.map((view) =>
                    // We make the entity key part of the view key so
                    // the element gets completely reset when the entity
                    // changes.
                    <EntityView key={`${view.id} ${entityKey(props.value.entity)}`}
                        view={view} current={view.id == props.viewID} value={props.value} onSelect={props.onSelect} />
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
    // TODO: This appears to not work when the view is display: none, so
    // only the current view successfully scrolls.
    const domRef = useRef<HTMLDivElement>(null);
    useEffect(() => {
        if (domRef.current !== null) {
            const first = domRef.current.querySelector(".ob-selected");
            first?.scrollIntoView({ behavior: "smooth", block: "nearest" });
        }
    }, [props.value]);

    const View = props.view;
    // The middle div controls visibility and creates a separate scroll
    // region for each view.
    return (<div ref={domRef} style={{ display: props.current ? "block" : "none" }}>
        {/* The inner div creates padding within the scroll region */}
        <div className="p-3">
            <View.element value={props.value} onSelect={props.onSelect}></View.element>
        </div>
    </div>);
}
