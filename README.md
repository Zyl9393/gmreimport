# GameMaker Sprite Re-Import (`gmreimport`)

This program, `gmreimport`, will scan a GameMaker project's `sprites` directory for sprites and their subimages (frames) and look in a specified directory for `.png` files to update them with and copy them over, effectively performing a re-import of all sprite assets (for which their source files could be found).

This is a console application which needs to be invoked from a command line. An easy way to accomplish this is through use of a batch-script (`.bat`-file).

Recommended `.bat`-files:

`reimport-sprites-dry.bat`: (dry run logs what `gmreimport` would do, without actually doing it, making it safe to run)
```
gmreimport.exe -dry -src .\sprites_master -dst .\mygame\sprites
@ECHO Press any key to close this window.
@PAUSE>NUL
```

`reimport-sprites.bat`: (writes files in the directory given after `-dst`; **absolutely backup your entire project beforehand**)
```
gmreimport.exe -src .\sprites_master -dst .\mygame\sprites
@ECHO Press any key to close this window.
@PAUSE>NUL
```

Path following `-dst` needs to point to the `sprites`-folder of a GameMaker project and will be scanned for existing sprites.  
Path following `-src` needs to point to the directory containing updated `.png` files.

## Why?

Image editing software other than GameMaker's built-in sprite editor exists, but there is no built-in way to quickly reimport thousands of files.

## How?

The names of GameMaker sprite assets are in the same *namespace*, meaning even if two sprites are in different groups or directories, they still must be distinct, i.e. have different names, or GameMaker will act up. We use this fact to enable updating any sprite in a given GameMaker project with a `.png` found in a given source directory (ideally your source files directory containing `.png` files and possibly their lossless masters such as `.aseprite`, `.clip`, `.psd`, etc.) or any of its subdirectories.

If GameMaker has not recoded any previously re-imported `.png`-files, `gmreimport` will not touch them, preventing unnecessary drive writes.

## Requirements

### Source File Name

#### For non-animated sprites

The names of image files for non-animated sprites must match the sprite name, including the `spr_`-prefix (if they have it), followed by the `.png` extension (casing in the extension may vary). For example a sprite `spr_cheese` can only be re-imported from a file called `spr_cheese.png`.

#### For animated sprites

The names of image files for animated sprites must match the sprite name, including the `spr_`-prefix (if they have it), followed by a numbering scheme and then the `.png` extension (casing in the extension may vary). For example, the third frame of a sprite `spr_feather` can be re-imported through a file called `spr_feather_3.png` (1-based index) or `spr_feather2.png` (0-based index, no underscore). `gmreimport` will use the first of the following schemes with which the first frame's source image file can be found:

* `_0000`
* `_000`
* `_00`
* `_0`
* `0000`
* `000`
* `00`
* `0`
* `_0001`
* `_001`
* `_01`
* `_1`
* `0001`
* `001`
* `01`
* `1`

In the above example, `spr_feather2.png` would be chosen because it implies the existence of a 0-based scheme starting with `spr_feather0.png` and all 0-based schemes take precedence over all 1-based schemes.

### No GameMaker Layers

Sprites which have more than one layer in GameMaker's built-in sprite editor are not considered for re-import as this implies you have done work on them which does not exist outside of GameMaker and thus is not desired to be overwritten.

### Sprite must already exist

`gmreimport` does not currently perform any initial import. It will only touch those sprites which already exist in your project.

## Caveats

### PNG Recoding

GameMaker likes to recode the `.png`-files of sprites. We don't know how or why, but it does not seem to be necessary. The only issue this is known to cause is that `gmreimport` will fail to detect when recoded files are in reality unchanged, updating them unnecessarily. If you ever notice strange graphical behaviors or artifacts, or an increase in loading times after using `gmreimport`, the specific encoding done by your image editing software might have a compatibility issue with how GameMaker attempts to decode it. So far, I can report `.png`-files exported by Aseprite and Clip Studio not causing any problems.

### Changed files warning

When running `gmreimport` while the project is open in GameMaker, GameMaker might prompt you about changes to files. Strangely, when selecting `Reload` it will recode the newly imported `.png` files, but clicking `Save` will accept them as-is. Thus, `Save` is generally preferred if you encounter this, as `gmreimport` can then avoid unnecessary re-import of files that have not changed. However, also notice hint about recoding above.

### Rigid naming pattern

You might find the naming pattern enforced by this tool to be restrictive. Perhaps you would like your sprites to have the `spr_`-prefix, but not your image files, or maybe you prefer to construct the sprite name through concatenation with a parent folder name, etc.  
I have found that keeping the names of source files as close to the sprite name as possible provides several advantages:
* You can freely reorganize your source files into different folders without any renaming work.
* Matches the behavior of drag &amp; drop in GameMaker which also morphs file names right into sprite names.
* Matches single namespace paradigm which GameMaker uses, enabling us to find source files no matter where in your folder structure they are and without `gmreimport` needing to know the semantics of your folder structure.
