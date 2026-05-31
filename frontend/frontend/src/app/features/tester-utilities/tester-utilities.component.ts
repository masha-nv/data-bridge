import { CommonModule } from '@angular/common';
import { ClipboardModule } from '@angular/cdk/clipboard';
import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  OnInit,
  inject,
  signal,
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { FormBuilder, ReactiveFormsModule } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatCardModule } from '@angular/material/card';
import { MatCheckboxModule } from '@angular/material/checkbox';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatTabsModule } from '@angular/material/tabs';
import { ActivatedRoute, Router } from '@angular/router';

@Component({
  selector: 'app-tester-utilities',
  standalone: true,
  imports: [
    ClipboardModule,
    CommonModule,
    ReactiveFormsModule,
    MatButtonModule,
    MatCardModule,
    MatCheckboxModule,
    MatFormFieldModule,
    MatInputModule,
    MatTabsModule,
  ],
  templateUrl: './tester-utilities.component.html',
  styleUrl: './tester-utilities.component.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class TesterUtilitiesComponent implements OnInit {
  private readonly destroyRef = inject(DestroyRef);
  private readonly formBuilder = inject(FormBuilder);
  private readonly route = inject(ActivatedRoute);
  private readonly router = inject(Router);
  private payloadCopiedResetId: number | null = null;

  readonly selectedTabIndex = signal(0);
  readonly payloadCopied = signal(false);
  readonly payloadPreview = `{
  "status": "pending-sample-payload",
  "details": "Payload structure will be added after the sample JSON contract is provided."
}`;
  readonly identifiedBenesForm = this.formBuilder.nonNullable.group({
    inHospice: false,
    hasEsrd: false,
    ageBand: '',
    additionalAttributes: '',
  });

  ngOnInit(): void {
    this.route.queryParamMap
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe((queryParams) => {
        this.selectedTabIndex.set(
          this.getTabIndex(queryParams.get('tab') ?? 'identified-benes'),
        );
      });
  }

  handleTabChange(index: number): void {
    const tab = this.getTabName(index);
    void this.router.navigate([], {
      relativeTo: this.route,
      queryParams: { tab },
      queryParamsHandling: 'merge',
    });
  }

  resetIdentifiedBenesFilters(): void {
    this.identifiedBenesForm.reset({
      inHospice: false,
      hasEsrd: false,
      ageBand: '',
      additionalAttributes: '',
    });
    this.identifiedBenesForm.markAsPristine();
    this.identifiedBenesForm.markAsUntouched();
  }

  handlePayloadCopied(copied: boolean): void {
    this.payloadCopied.set(copied);

    if (this.payloadCopiedResetId !== null) {
      window.clearTimeout(this.payloadCopiedResetId);
      this.payloadCopiedResetId = null;
    }

    if (copied) {
      this.payloadCopiedResetId = window.setTimeout(() => {
        this.payloadCopied.set(false);
        this.payloadCopiedResetId = null;
      }, 2000);
    }
  }

  private getTabIndex(tab: string): number {
    switch (tab) {
      case 'locked-benes':
        return 1;
      default:
        return 0;
    }
  }

  private getTabName(index: number): string {
    switch (index) {
      case 1:
        return 'locked-benes';
      default:
        return 'identified-benes';
    }
  }
}