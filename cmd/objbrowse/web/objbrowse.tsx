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

export function App(props: AppProps) {
    // TODO: Sync current entity (and selected range in that entity) to the URL
    // history.
    const [selected, setSelected] = useState<Selection | undefined>(undefined);
    const [view, setView] = useState("");

    const setEntity = (entity?: Entity) => {
        if (entity === undefined) {
            setSelected(undefined);
            setView("");
        } else {
            setSelected({ entity, ranges: new Ranges() });
            // TODO: Not all views are valid. Select the first valid one for this entity.
            setView(props.views[0].id);
        }
    }

    return (
        <div className="ob-root">
            <div className="container-fluid">
                <div className="row flex-xl-nowrap">
                    <div className="col-2 p-0">
                        <SymPanel entity={selected?.entity} onSelect={setEntity} />
                    </div>
                    {selected !== undefined &&
                        <div className="col-10 p-0">
                            <EntityPanel views={props.views} view={view} onSelectView={setView} value={selected} onSelect={setSelected} />
                        </div>
                    }
                </div>
            </div>
        </div>
    );
}

interface SymPanelProps { entity?: Entity, onSelect: (ent?: Entity) => void }

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
            <SymList filter={filter} entity={props.entity} onSelect={props.onSelect} />
        </div>
    </div >);
}

interface SymListProps { filter: RegExp, entity?: Entity, onSelect: (ent?: Entity) => void }

function SymList(props: SymListProps) {
    // Scroll when the selected entity changes.
    const selectedElt = useRef<HTMLLIElement>(null);
    useEffect(() => {
        if (selectedElt.current !== null) {
            selectedElt.current.scrollIntoView({ block: "nearest" });
        }
    }, [props.entity]);

    // Fetch symbol list.
    const symsJSON = useFetchJSON("/syms");
    if (symsJSON.pending) {
        return <div className="mx-3">{symsJSON.pending}</div>;
    }
    const syms: string[] = symsJSON.value;

    // TODO: Indicate symbol type? Perhaps to the left of the symbol?
    //
    // TODO: Display columns with much more symbol information? I would probably
    // have to limit the symbol name length and make the SymPanel expandable.
    return (
        <ul className="ob-symlist list-unstyled text-nowrap">
            {syms.map((name, id) => {
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
    view: string;
    onSelectView: (view: string) => void;
    value: Selection;
    onSelect: (sel: Selection) => void;
}

function EntityPanel(props: EntityPanelProps) {
    // TODO: This just loops over all the views, but I need to present
    // just the views that support the current entity.
    return (
        <div className="ob-view-container">
            {/* padding-left extends the bottom border to the left */}
            {/* padding-top keeps it in place when scrolling */}
            <nav className="nav nav-tabs ps-3 pt-3">
                {props.views.map((View) =>
                    View.id == props.view ?
                        <span className="nav-link active" aria-current="page">{View.label}</span> :
                        <span className="nav-link" onClick={() => props.onSelectView(View.id)}>{View.label}</span>
                )}
            </nav>
            {props.views.map((View) =>
                // We make the entity key part of the view key so
                // the element gets completely reset when the entity
                // changes.
                <div key={`${View.id} ${entityKey(props.value.entity)}`} className="p-3" style={{ display: View.id == props.view ? "block" : "none" }}>
                    <View.element value={props.value} onSelect={props.onSelect}></View.element>
                </div>
            )}
        </div>
    );
}
