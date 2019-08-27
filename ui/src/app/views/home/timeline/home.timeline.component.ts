import { ChangeDetectionStrategy, ChangeDetectorRef, Component, OnInit } from '@angular/core';
import { TranslateService } from '@ngx-translate/core';
import { EventService } from 'app/event.service';
import { Event } from 'app/model/event.model';
import { PipelineStatus } from 'app/model/pipeline.model';
import { ProjectFilter, TimelineFilter } from 'app/model/timeline.model';
import { TimelineStore } from 'app/service/timeline/timeline.store';
import { AutoUnsubscribe } from 'app/shared/decorator/autoUnsubscribe';
import { ToastService } from 'app/shared/toast/ToastService';
import { finalize, first } from 'rxjs/operators';
import { Subscription } from 'rxjs/Subscription';

@Component({
    selector: 'app-home-timeline',
    templateUrl: './home.timeline.html',
    styleUrls: ['./home.timeline.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush
})
@AutoUnsubscribe()
export class HomeTimelineComponent implements OnInit {

    loading = true;
    events: Array<Event>;

    timelineSub: Subscription;
    selectedTab = 'timeline';

    currentItem = 0;
    pipelineStatus = PipelineStatus;

    filter: TimelineFilter;
    filterSub: Subscription;

    constructor(private _timelineStore: TimelineStore,
        private _translate: TranslateService,
        private _toast: ToastService,
        private _eventService: EventService,
        private _cd: ChangeDetectorRef
    ) { }

    ngOnInit(): void {
        this.filterSub = this._timelineStore.getFilter().subscribe(f => {
            this.filter = f;
            this._eventService.initFilter(this.filter);
            if (this.timelineSub) {
                this.timelineSub.unsubscribe();
            }
            if (f) {
                this.timelineSub = this._timelineStore.alltimeline()
                    .subscribe(es => {
                        if (!es) {
                            return;
                        }
                        this.loading = false;
                        this.events = es.toArray();
                        this.currentItem = this.events.length;
                        this._cd.markForCheck();
                    });
            }
            this._cd.markForCheck();
        });
    }

    onScroll() {
        this._timelineStore.getMore(this.currentItem + 1, false);
    }

    addFilter(e: Event): void {
        if (!this.filter.projects) {
            this.filter.projects = new Array<ProjectFilter>();
        }
        let pFilter = this.filter.projects.find(p => p.key === e.project_key);
        if (!pFilter) {
            pFilter = new ProjectFilter();
            pFilter.key = e.project_key;
            this.filter.projects.push(pFilter);
        }

        if (!pFilter.workflow_names) {
            pFilter.workflow_names = new Array<string>();
        }
        let wName = pFilter.workflow_names.find(w => w === e.workflow_name);
        if (!wName) {
            pFilter.workflow_names.push(e.workflow_name);
        }
        this._timelineStore.saveFilter(this.filter)
            .pipe(first(), finalize(() => this._cd.markForCheck()))
            .subscribe(() => {
                this._toast.success('', this._translate.instant('timeline_filter_updated'));
            });
    }
}
