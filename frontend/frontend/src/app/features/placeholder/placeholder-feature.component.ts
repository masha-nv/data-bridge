import { CommonModule } from '@angular/common';
import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { ActivatedRoute } from '@angular/router';
import { map } from 'rxjs';
import { toSignal } from '@angular/core/rxjs-interop';

@Component({
  selector: 'app-placeholder-feature',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './placeholder-feature.component.html',
  styleUrls: ['./placeholder-feature.component.scss'],
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class PlaceholderFeatureComponent {
  private readonly route = inject(ActivatedRoute);

  readonly title = toSignal(
    this.route.data.pipe(map((data) => (data['title'] as string) ?? 'Feature')),
    { initialValue: 'Feature' },
  );
}